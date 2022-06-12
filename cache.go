package cache

import (
	"context"
	"reflect"
	"time"

	"golang.org/x/sync/singleflight"
)

type cache struct {
	configs       map[string]*config
	onCacheHit    func(prefix string, key string, count int)
	onCacheMiss   func(prefix string, key string, count int)
	onLCCostAdd   func(key string, cost int)
	onLCCostEvict func(key string, cost int)
	mb            *messageBroker

	singleflight singleflight.Group
}

type config struct {
	shared    Adapter
	local     Adapter
	sharedTTL time.Duration
	localTTL  time.Duration
	mGetter   MGetterFunc
	marshal   MarshalFunc
	unmarshal UnmarshalFunc
}

func (c *cache) GetByFunc(ctx context.Context, prefix, key string, container interface{}, getter OneTimeGetterFunc) error {
	cfg, ok := c.configs[prefix]
	if !ok {
		return ErrPfxNotRegistered
	}

	intf, err, _ := c.singleflight.Do(getCacheKey(prefix, key), func() (interface{}, error) {
		cacheKey := getCacheKey(prefix, key)
		cacheVals, err := c.load(ctx, cfg, cacheKey)
		if err != nil {
			return nil, err
		}

		// cache hit
		if cacheVals[0].Valid {
			c.onCacheHit(prefix, key, 1)
			return cacheVals[0].Bytes, nil
		}

		// cache missed once
		c.onCacheMiss(prefix, key, 1)

		// using oneTimeGetter to implement Cache-Aside pattern
		intf, err := getter()
		if err != nil {
			return nil, err
		}

		b, err := cfg.marshal(intf)
		if err != nil {
			return nil, err
		}

		// refill cache
		if err := c.refill(ctx, cfg, map[string][]byte{cacheKey: b}); err != nil {
			return nil, err
		}

		return b, nil
	})

	if err != nil {
		return err
	}

	return cfg.unmarshal(intf.([]byte), container)
}

func (c *cache) Get(ctx context.Context, prefix, key string, container interface{}) error {
	intf, err, _ := c.singleflight.Do(getCacheKey(prefix, key), func() (interface{}, error) {
		return c.MGet(ctx, prefix, key)
	})
	if err != nil {
		return err
	}

	return intf.(Result).Get(ctx, 0, container)
}

func (c *cache) MGet(ctx context.Context, prefix string, keys ...string) (Result, error) {
	cfg, ok := c.configs[prefix]
	if !ok {
		return nil, ErrPfxNotRegistered
	}

	if len(keys) == 0 {
		return &result{unmarshal: cfg.unmarshal}, nil
	}

	// TODO: support singleflight in the future

	// IdxM means internal index map
	// dKeys means deduped keys
	IdxM, dKeys := dedup(keys)

	res := &result{
		internalIdx: IdxM,
		vals:        make([][]byte, len(dKeys)),
		errs:        make([]error, len(dKeys)),
		unmarshal:   cfg.unmarshal,
	}

	// 1. get from cache
	keyIdx := getKeyIndex(dKeys)
	cacheKeys := getCacheKeys(prefix, dKeys)

	cacheVals, err := c.load(ctx, cfg, cacheKeys...)
	if err != nil {
		return nil, err
	}

	missKeys := []string{}
	for i, k := range dKeys {
		if !cacheVals[i].Valid {
			missKeys = append(missKeys, k)
			res.errs[i] = ErrCacheMiss
			c.onCacheMiss(prefix, k, 1)
			continue
		}

		res.vals[i] = cacheVals[i].Bytes
		c.onCacheHit(prefix, k, 1)
	}

	// no cache missing
	if len(missKeys) == 0 {
		return res, nil
	}

	// no mGetter, simple Get & Set pattern, return it directly
	if cfg.mGetter == nil {
		return res, nil
	}

	// 2. using mGetter to implement Cache-Aside pattern
	intfs, err := cfg.mGetter(missKeys...)
	if err != nil {
		return nil, err
	}

	vs := reflect.ValueOf(intfs)
	if vs.Kind() != reflect.Slice {
		return nil, ErrMGetterResponseNotSlice
	}
	if vs.Len() != len(missKeys) {
		return nil, ErrMGetterResponseLengthInvalid
	}

	m := map[string][]byte{}
	for i, mk := range missKeys {
		v := vs.Index(i).Interface()
		b, err := cfg.marshal(v)
		if err != nil {
			res.errs[keyIdx[mk]] = err
			continue
		}

		m[getCacheKey(prefix, mk)] = b
		res.vals[keyIdx[mk]] = b
		res.errs[keyIdx[mk]] = nil
	}

	// 3. load the cache
	c.refill(ctx, cfg, m)

	return res, nil
}

func (c *cache) Del(ctx context.Context, prefix string, keys ...string) error {
	cfg, ok := c.configs[prefix]
	if !ok {
		return ErrPfxNotRegistered
	}

	if len(keys) == 0 {
		return nil
	}

	return c.del(ctx, cfg, getCacheKeys(prefix, keys)...)
}

func (c *cache) Set(ctx context.Context, prefix string, key string, value interface{}) error {
	return c.MSet(ctx, prefix, map[string]interface{}{key: value})
}

func (c *cache) MSet(ctx context.Context, prefix string, keyValues map[string]interface{}) error {
	cfg, ok := c.configs[prefix]
	if !ok {
		return ErrPfxNotRegistered
	}

	m := map[string][]byte{}
	for k, value := range keyValues {
		b, err := cfg.marshal(value)
		if err != nil {
			return err
		}

		m[getCacheKey(prefix, k)] = b
	}

	return c.refill(ctx, cfg, m)
}

func getKeyIndex(keys []string) map[string]int {
	keyIdx := map[string]int{}
	for i, k := range keys {
		keyIdx[k] = i
	}

	return keyIdx
}

func dedup(params []string) (map[int]int, []string) {
	if len(params) == 1 {
		return map[int]int{0: 0}, params
	}

	dedupedKeys := []string{}
	// dedupedIdx is an indirect index that maps un-dedup idx to dedup idx
	dedupedIdx := map[int]int{}
	// m maps param to dedup idx
	m := map[string]int{}
	for i, param := range params {
		if _, ok := m[param]; ok {
			dedupedIdx[i] = m[param]
			continue
		}

		dedupedIdx[i] = len(dedupedKeys)
		m[param] = len(dedupedKeys)
		dedupedKeys = append(dedupedKeys, param)
	}

	return dedupedIdx, dedupedKeys
}

// load loads data from cache, and refill it if necessary
func (c *cache) load(ctx context.Context, cfg *config, keys ...string) ([]Value, error) {
	vals := make([]Value, len(keys))
	missKeys := make([]string, len(keys))
	copy(missKeys, keys)

	keyIdx := getKeyIndex(keys)

	// 1. load from local cache
	if cfg.local != nil {
		// allow the failure when getting local cache
		vals, _ = cfg.local.MGet(ctx, keys)

		missKeys = []string{}
		for i, val := range vals {
			if !val.Valid {
				missKeys = append(missKeys, keys[i])
			}
		}
	}

	// no cache missing
	if len(missKeys) == 0 {
		return vals, nil
	}

	// 2. load from shared cache
	if cfg.shared != nil {
		missVals, err := cfg.shared.MGet(ctx, missKeys)
		if err != nil {
			return nil, err
		}

		// refill missing values into vals
		for i, mVal := range missVals {
			vals[keyIdx[missKeys[i]]] = mVal
		}
	}

	// 3. refill the local cache if possible
	if cfg.local != nil {
		m := map[string][]byte{}
		for _, k := range keys {
			val := vals[keyIdx[k]]
			if val.Valid {
				m[k] = val.Bytes
			}
		}

		if len(m) != 0 {
			cfg.local.MSet(ctx, m, cfg.localTTL,
				WithOnCostAddFunc(c.onLCCostAdd),
				WithOnCostEvictFunc(c.onLCCostEvict),
			)

			c.evictRemoteKeyMap(ctx, m)
		}
	}

	return vals, nil
}

// refill refills the cache with given keyBytes
func (c *cache) refill(ctx context.Context, cfg *config, keyBytes map[string][]byte) error {
	// set shared cache first if necessary
	if cfg.shared != nil {
		if err := cfg.shared.MSet(ctx, keyBytes, cfg.sharedTTL); err != nil {
			return err
		}
	}

	// then, set local cache if necessary
	if cfg.local != nil {
		if err := cfg.local.MSet(ctx, keyBytes, cfg.localTTL,
			WithOnCostAddFunc(c.onLCCostAdd),
			WithOnCostEvictFunc(c.onLCCostEvict),
		); err != nil {
			return nil
		}

		c.evictRemoteKeyMap(ctx, keyBytes)
	}

	return nil
}

func (c *cache) del(ctx context.Context, cfg *config, keys ...string) error {
	if cfg.shared != nil {
		if err := cfg.shared.Del(ctx, keys...); err != nil {
			return err
		}
	}

	if cfg.local != nil {
		if err := cfg.local.Del(ctx, keys...); err != nil {
			return err
		}

		c.evictRemoteKeys(ctx, keys...)
	}

	return nil
}

func (c *cache) evictRemoteKeyMap(ctx context.Context, keyM map[string][]byte) error {
	if !c.mb.registered() {
		// no pubsub, do nothing
		return nil
	}

	keys := make([]string, len(keyM))
	i := 0
	for k := range keyM {
		keys[i] = k
		i++
	}

	return c.evictRemoteKeys(ctx, keys...)
}

func (c *cache) evictRemoteKeys(ctx context.Context, keys ...string) error {
	if !c.mb.registered() {
		// no pubsub, do nothing
		return nil
	}

	return c.mb.send(ctx, event{
		Type: EventTypeEvict,
		Body: eventBody{Keys: keys},
	})
}

type result struct {
	internalIdx map[int]int
	vals        [][]byte
	errs        []error
	unmarshal   UnmarshalFunc
}

func (r *result) Len() int {
	return len(r.internalIdx)
}

func (r *result) Get(ctx context.Context, idx int, container interface{}) error {
	if idx < 0 || idx >= r.Len() {
		return ErrResultIndexInvalid
	}

	if r.errs[r.internalIdx[idx]] != nil {
		return r.errs[r.internalIdx[idx]]
	}

	return r.unmarshal(r.vals[r.internalIdx[idx]], container)
}
