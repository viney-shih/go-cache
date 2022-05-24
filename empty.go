package cache

import (
	"context"
	"time"
)

// NewEmpty generates Adapter without implementation
func NewEmpty() Adapter {
	return &empty{}
}

type empty struct{}

func (adp *empty) MSet(ctx context.Context, keyItems map[string][]byte, ttl time.Duration, options ...MSetOptions) error {
	// do nothing
	return nil
}

func (adp *empty) MGet(ctx context.Context, keys []string) ([]Value, error) {
	// do nothing
	return make([]Value, len(keys)), nil
}

func (adp *empty) Del(ctx context.Context, keys ...string) error {
	// do nothing
	return nil
}
