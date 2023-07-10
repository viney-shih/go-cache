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
	maxOffset     = 10 * time.Second
	defaultOffset = -1
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
	// samples are the number of keys to track frequency
	// TinyLFU works best for small number of keys (~ 100k)
	// Ref: https://github.com/vmihailenco/go-cache-benchmark
	// consider the discussing in (https://github.com/ben-manes/caffeine/issues/106),
	// choose ~10x the cache size as the default value.
	samples := size * 10

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

func (lfu *tinyLFU) MSet(
	ctx context.Context, keyVals map[string][]byte, ttl time.Duration, options ...MSetOptions,
) error {
	if len(keyVals) == 0 {
		return nil
	}

	// load options
	o := loadMSetOptions(options...)
	// offset is used to adjust the ttl preventing expiring at the same time
	offset := lfu.offset
	if offset == defaultOffset {
		offset = ttl / 10
		if offset > maxOffset {
			offset = maxOffset
		}
	}

	lfu.mut.Lock()
	defer lfu.mut.Unlock()

	for key, b := range keyVals {
		t := ttl
		if offset > 0 {
			t += time.Duration(lfu.rand.Int63n(int64(offset)))
		}

		cost := len(b)
		if o.onCostAdd != nil {
			o.onCostAdd(ctx, key, cost)
		}

		lfu.lfu.Set(&tinylfu.Item{
			Key:      key,
			Value:    b,
			ExpireAt: time.Now().Add(t),
			OnEvict: func() {
				if o.onCostEvict != nil {
					o.onCostEvict(ctx, key, cost)
				}
			},
		})
	}

	return nil
}

func (lfu *tinyLFU) MGet(ctx context.Context, keys []string) ([]Value, error) {
	lfu.mut.Lock()
	defer lfu.mut.Unlock()

	vals := make([]Value, len(keys))
	for i, key := range keys {
		val, ok := lfu.lfu.Get(key)
		if !ok {
			vals[i] = Value{Valid: false, Bytes: nil}
			continue
		}

		b, ok := val.([]byte)
		vals[i] = Value{Valid: ok, Bytes: b}
	}

	return vals, nil
}

func (lfu *tinyLFU) Del(ctx context.Context, keys ...string) error {
	lfu.mut.Lock()
	defer lfu.mut.Unlock()

	for _, key := range keys {
		lfu.lfu.Del(key)
	}

	return nil
}
