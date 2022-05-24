package cache

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

var (
	// usedPrefixs records the prefixes registered before
	usedPrefixs = map[string]struct{}{}
)

func newFactory(sharedCache Adapter, localCache Adapter, options ...ServiceOptions) Factory {
	// load options
	o := loadServiceOptions(options...)
	// need to specify marshalFunc and unmarshalFunc at the same time
	if o.marshalFunc == nil && o.unmarshalFunc != nil {
		panic(errors.New("both of Marshal and Unmarshal functions need to be specified"))
	} else if o.marshalFunc != nil && o.unmarshalFunc == nil {
		panic(errors.New("both of Marshal and Unmarshal functions need to be specified"))
	}

	var marshalFunc MarshalFunc
	var unmarshalFunc UnmarshalFunc
	marshalFunc = json.Marshal
	unmarshalFunc = json.Unmarshal

	if o.marshalFunc != nil {
		marshalFunc = o.marshalFunc
	}
	if o.unmarshalFunc != nil {
		unmarshalFunc = o.unmarshalFunc
	}

	f := &factory{
		sharedCache:   sharedCache,
		localCache:    localCache,
		pubsub:        o.pubsub,
		marshal:       marshalFunc,
		unmarshal:     unmarshalFunc,
		onCacheHit:    o.onCacheHit,
		onCacheMiss:   o.onCacheMiss,
		onLCCostAdd:   o.onLCCostAdd,
		onLCCostEvict: o.onLCCostEvict,
	}

	// subscribing if necessary
	f.subscribeEvictEvents(context.TODO())

	return f
}

type factory struct {
	sharedCache Adapter
	localCache  Adapter
	pubsub      Pubsub

	marshal       MarshalFunc
	unmarshal     UnmarshalFunc
	onCacheHit    func(prefix string, key string, count int)
	onCacheMiss   func(prefix string, key string, count int)
	onLCCostAdd   func(prefix string, key string, cost int)
	onLCCostEvict func(prefix string, key string, cost int)

	closeOnce sync.Once
	wg        sync.WaitGroup
}

func (f *factory) NewCache(settings []Setting) Cache {
	m := map[string]*config{}
	for _, setting := range settings {
		// check prefix
		if setting.Prefix == "" {
			panic(errors.New("not allowed empty prefix"))
		}
		if _, ok := usedPrefixs[setting.Prefix]; ok {
			panic(errors.New("duplicated prefix"))
		}
		usedPrefixs[setting.Prefix] = struct{}{}

		cfg := &config{
			mGetter:   setting.MGetter,
			marshal:   f.marshal,
			unmarshal: f.unmarshal,
		}

		// need to specify marshalFunc and unmarshalFunc at the same time
		if setting.MarshalFunc == nil && setting.UnmarshalFunc != nil {
			panic(errors.New("both of Marshal and Unmarshal functions need to be specified"))
		} else if setting.MarshalFunc != nil && setting.UnmarshalFunc == nil {
			panic(errors.New("both of Marshal and Unmarshal functions need to be specified"))
		}

		if setting.MarshalFunc != nil {
			cfg.marshal = setting.MarshalFunc
		}
		if setting.UnmarshalFunc != nil {
			cfg.unmarshal = setting.UnmarshalFunc
		}

		for typ, attr := range setting.CacheAttributes {
			if typ == SharedCacheType {
				cfg.shared = f.sharedCache
				cfg.sharedTTL = attr.TTL
			} else if typ == LocalCacheType {
				cfg.local = f.localCache
				cfg.localTTL = attr.TTL
			}
		}

		// need to indicate at least one cache type
		if cfg.shared == nil && cfg.local == nil {
			panic(errors.New("no cache type indicated"))
		}

		m[setting.Prefix] = cfg
	}

	return &cache{
		configs: m,
		pubsub:  f.pubsub,
		onCacheHit: func(prefix string, key string, count int) {
			// trigger the callback on cache hitted if necessary
			if f.onCacheHit != nil {
				f.onCacheHit(prefix, key, count)
			}
		},
		onCacheMiss: func(prefix string, key string, count int) {
			// trigger the callback on cache missed if necessary
			if f.onCacheMiss != nil {
				f.onCacheMiss(prefix, key, count)
			}
		},
		onLCCostAdd: func(cKey string, cost int) {
			// trigger the callback on local cache added if necessary
			if f.onLCCostAdd != nil {
				pfx, key := getPrefixAndKey(cKey)
				f.onLCCostAdd(pfx, key, cost)
			}
		},
		onLCCostEvict: func(cKey string, cost int) {
			// trigger the callback on local cache evicted if necessary
			if f.onLCCostEvict != nil {
				pfx, key := getPrefixAndKey(cKey)
				f.onLCCostEvict(pfx, key, cost)
			}
		},
	}
}

func (f *factory) Close() {
	f.closeOnce.Do(func() {
		// close subscribing
		f.pubsub.Close()
		// wait for all goroutines stopped
		f.wg.Wait()
	})
}

func (f *factory) subscribeEvictEvents(ctx context.Context) {
	if f.pubsub == nil || f.localCache == nil {
		// do nothing
		return
	}

	f.wg.Add(1)
	go func() {
		defer f.wg.Done()

		// listen to evicting key events
		for mess := range f.pubsub.Sub(ctx, evictTopic) {
			cKey := string(mess.Content())
			f.localCache.Del(ctx, cKey)
		}
	}()
}
