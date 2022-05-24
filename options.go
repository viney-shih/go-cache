package cache

// MarshalFunc specifies the algorithm during marshaling the value to bytes.
// The default is json.Marshal.
type MarshalFunc func(interface{}) ([]byte, error)

// UnmarshalFunc specifies the algorithm during unmarshaling the bytes to the value.
// The default is json.Unmarshal
type UnmarshalFunc func([]byte, interface{}) error

// CallbackFunc means the callback function triggered at the specified moment.
type CallbackFunc func(prefix string, key string, value interface{})

// ServiceOptions is an alias for functional argument.
type ServiceOptions func(opts *serviceOptions)

// serviceOptions contains all options which will be applied when calling New().
type serviceOptions struct {
	marshalFunc   MarshalFunc
	unmarshalFunc UnmarshalFunc
	onCacheHit    CallbackFunc
	onCacheMiss   CallbackFunc
	onLCCostAdd   CallbackFunc
	onLCCostEvict CallbackFunc
}

// WithMarshalFunc sets up the specified marshal funciton.
// Needs to consider with unmarshal function at the same time.
func WithMarshalFunc(f MarshalFunc) ServiceOptions {
	return func(opts *serviceOptions) {
		opts.marshalFunc = f
	}
}

// WithUnmarshalFunc sets up the specified unmarshal funciton.
// Needs to consider with marshal function at the same time.
func WithUnmarshalFunc(f UnmarshalFunc) ServiceOptions {
	return func(opts *serviceOptions) {
		opts.unmarshalFunc = f
	}
}

// OnCacheHitFunc sets up the callback function on cache hitted
func OnCacheHitFunc(f CallbackFunc) ServiceOptions {
	return func(opts *serviceOptions) {
		opts.onCacheHit = f
	}
}

// OnCacheMissFunc sets up the callback function on cache missed
func OnCacheMissFunc(f CallbackFunc) ServiceOptions {
	return func(opts *serviceOptions) {
		opts.onCacheMiss = f
	}
}

// OnLocalCacheCostAddFunc sets up the callback function on adding the cost of key in local cache
func OnLocalCacheCostAddFunc(f CallbackFunc) ServiceOptions {
	return func(opts *serviceOptions) {
		opts.onLCCostAdd = f
	}
}

// OnLocalCacheCostEvictFunc sets up the callback function on evicting the cost of key in local cache
func OnLocalCacheCostEvictFunc(f CallbackFunc) ServiceOptions {
	return func(opts *serviceOptions) {
		opts.onLCCostEvict = f
	}
}

func loadServiceOptions(options ...ServiceOptions) *serviceOptions {
	opts := &serviceOptions{}
	for _, option := range options {
		option(opts)
	}

	return opts
}
