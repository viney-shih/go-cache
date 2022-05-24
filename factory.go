package cache

import (
	"encoding/json"
	"errors"
)

const (
	packageKey = "ca"
	delimiter  = ":"
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

	return &factory{
		sharedCache:   sharedCache,
		localCache:    localCache,
		marshal:       marshalFunc,
		unmarshal:     unmarshalFunc,
		onCacheHit:    o.onCacheHit,
		onCacheMiss:   o.onCacheMiss,
		onLCCostAdd:   o.onLCCostAdd,
		onLCCostEvict: o.onLCCostEvict,
	}
}

type factory struct {
	sharedCache Adapter
	localCache  Adapter

	marshal       MarshalFunc
	unmarshal     UnmarshalFunc
	onCacheHit    CallbackFunc
	onCacheMiss   CallbackFunc
	onLCCostAdd   CallbackFunc
	onLCCostEvict CallbackFunc
}

func (s *factory) NewCache(settings []Setting) Cache {
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
			marshal:   s.marshal,
			unmarshal: s.unmarshal,
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
				cfg.shared = s.sharedCache
				cfg.sharedTTL = attr.TTL
			} else if typ == LocalCacheType {
				cfg.local = s.localCache
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
		onCacheHit: func(prefix string, key string, value interface{}) {
			// trigger the callback on cache hitted if necessary
			if s.onCacheHit != nil {
				s.onCacheHit(prefix, key, value)
			}
		},
		onCacheMiss: func(prefix string, key string, value interface{}) {
			// trigger the callback on cache missed if necessary
			if s.onCacheMiss != nil {
				s.onCacheMiss(prefix, key, value)
			}
		},
		onLCCostAdd: func(cKey string, value int) {
			// trigger the callback on local cache added if necessary
			if s.onLCCostAdd != nil {
				pfx, key := getPrefixAndKey(cKey)
				s.onLCCostAdd(pfx, key, value)
			}
		},
		onLCCostEvict: func(cKey string, value int) {
			// trigger the callback on local cache evicted if necessary
			if s.onLCCostEvict != nil {
				pfx, key := getPrefixAndKey(cKey)
				s.onLCCostEvict(pfx, key, value)
			}
		},
	}
}
