package cache

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/vmihailenco/go-tinylfu"
	"golang.org/x/exp/rand"
)

const (
	maxOffset      = 10 * time.Second
	defaultSamples = 100000
	defaultOffset  = -1
)

type tinyLFU struct {
	lfu *tinylfu.T
	// tinyLFU is not thread-safe, it needs a lock
	mut    sync.Mutex
	rand   *rand.Rand
	offset time.Duration
}

// NewTinyLFU generates Adapter with tinylfu
func NewTinyLFU(size int, options ...TinyLFUOptions) Adapter {
	// number of keys to track frequency
	// TinyLFU works best for small number of keys (~ 100k)
	// Ref: https://github.com/vmihailenco/go-cache-benchmark
	samples := defaultSamples

	o := loadtinyLFUOptions(options...)
	if o.offset != defaultOffset && o.offset < 0 {
		panic(errors.New("invalid offset"))
	}

	return &tinyLFU{
		lfu:    tinylfu.New(size, samples),
		rand:   rand.New(rand.NewSource(uint64(time.Now().UnixNano()))),
		offset: o.offset,
	}
}

// TinyLFUOptions is an alias for functional argument.
type TinyLFUOptions func(opts *tinyLFUOptions)

// tinyLFUOptions contains all options which will be applied when calling New().
type tinyLFUOptions struct {
	offset time.Duration
}

// WithOffset sets up the offset which is used to randomize TTL preventing
// expiring at the same time.
func WithOffset(offset time.Duration) TinyLFUOptions {
	return func(opts *tinyLFUOptions) {
		opts.offset = offset
	}
}

func loadtinyLFUOptions(options ...TinyLFUOptions) *tinyLFUOptions {
	opts := &tinyLFUOptions{offset: defaultOffset}
	for _, option := range options {
		option(opts)
	}

	return opts
}

func (adp *tinyLFU) MSet(
	ctx context.Context, keyVals map[string][]byte, ttl time.Duration, options ...MSetOptions,
) error {
	if len(keyVals) == 0 {
		return nil
	}

	// load options
	o := loadMSetOptions(options...)
	// offset is used to adjust the ttl preventing expiring at the same time
	offset := adp.offset
	if offset == defaultOffset {
		offset = ttl / 10
		if offset > maxOffset {
			offset = maxOffset
		}
	}

	adp.mut.Lock()
	defer adp.mut.Unlock()

	for key, b := range keyVals {
		t := ttl
		if offset > 0 {
			t += time.Duration(adp.rand.Int63n(int64(offset)))
		}

		cost := len(b)
		if o.onCostAdd != nil {
			o.onCostAdd(key, cost)
		}

		adp.lfu.Set(&tinylfu.Item{
			Key:      key,
			Value:    b,
			ExpireAt: time.Now().Add(t),
			OnEvict: func() {
				if o.onCostEvict != nil {
					o.onCostEvict(key, cost)
				}
			},
		})
	}

	return nil
}

func (adp *tinyLFU) MGet(ctx context.Context, keys []string) ([]Value, error) {
	adp.mut.Lock()
	defer adp.mut.Unlock()

	vals := make([]Value, len(keys))
	for i, key := range keys {
		val, ok := adp.lfu.Get(key)
		if !ok {
			vals[i] = Value{Valid: false, Bytes: nil}
			continue
		}

		b, ok := val.([]byte)
		vals[i] = Value{Valid: ok, Bytes: b}
	}

	return vals, nil
}

func (adp *tinyLFU) Del(ctx context.Context, keys ...string) error {
	adp.mut.Lock()
	defer adp.mut.Unlock()

	for _, key := range keys {
		adp.lfu.Del(key)
	}

	return nil
}
