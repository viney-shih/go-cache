package cache

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

// NewRedis generates Adapter with go-redis
func NewRedis(ring *redis.Ring) Adapter {
	return &rds{
		ring: ring,
	}
}

type rds struct {
	ring *redis.Ring
}

func (adp *rds) MSet(
	ctx context.Context, keyVals map[string][]byte, ttl time.Duration, options ...MSetOptions,
) error {
	if len(keyVals) == 0 {
		return nil
	}

	_, err := adp.ring.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		// set multiple pairs
		pairSlice := make([]interface{}, len(keyVals)*2)
		i := 0
		for key, b := range keyVals {
			pairSlice[i] = key
			pairSlice[i+1] = b

			i += 2
		}

		pipe.MSet(ctx, pairSlice)

		// set expiration for each key
		for key := range keyVals {
			pipe.PExpire(ctx, key, ttl)
		}
		return nil
	})

	return err
}

func (adp *rds) MGet(ctx context.Context, keys []string) ([]Value, error) {
	vals, err := adp.ring.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	values := make([]Value, len(vals))
	for i, val := range vals {
		if val == nil {
			values[i] = Value{Valid: false, Bytes: nil}
			continue
		}

		s, ok := val.(string)
		if !ok {
			values[i] = Value{Valid: false, Bytes: nil}
			continue
		}

		values[i] = Value{Valid: ok, Bytes: []byte(s)}
	}

	return values, nil
}

func (adp *rds) Del(ctx context.Context, keys ...string) error {
	_, err := adp.ring.Del(ctx, keys...).Result()

	return err
}
