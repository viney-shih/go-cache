/*
 * @Author: zhuyaohui@zuoyebang.com
 * @Date: 2023-05-11 21:49:27
 * @LastEditors: zhuyaohui@zuoyebang.com
 * @LastEditTime: 2023-05-17 15:37:32
 * @FilePath: /go-cache/adapter.go
 * @Description:
 */
package cache

import (
	"context"
	"time"
)

// Adapter is the interface communicating with shared/local caches.
type Adapter interface {
	MGet(context context.Context, keys []string) ([]Value, error)
	MSet(context context.Context, keyVals map[string][]byte, ttl time.Duration, options ...MSetOptions) error
	Del(context context.Context, keys ...string) error
}

// MSetOptions is an alias for functional argument.
type MSetOptions func(opts *msetOptions)

type msetOptions struct {
	onCostAdd   func(ctx context.Context, key string, cost int)
	onCostEvict func(ctx context.Context, key string, cost int)
}

// WithOnCostAddFunc sets up the callback when adding the cache with key and cost.
func WithOnCostAddFunc(f func(ctx context.Context, key string, cost int)) MSetOptions {
	return func(opts *msetOptions) {
		opts.onCostAdd = f
	}
}

// WithOnCostEvictFunc sets up the callback when evicting the cache with key and cost.
func WithOnCostEvictFunc(f func(ctx context.Context, key string, cost int)) MSetOptions {
	return func(opts *msetOptions) {
		opts.onCostEvict = f
	}
}

func loadMSetOptions(options ...MSetOptions) *msetOptions {
	opts := &msetOptions{}
	for _, option := range options {
		option(opts)
	}

	return opts
}

// Value is returned by MGet()
type Value struct {
	// Valid stands for existing in cache or not.
	Valid bool
	// Bytes stands for the return value in byte format.
	Bytes []byte
}
