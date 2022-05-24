package cache

import (
	"context"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// Redis support two interface: Adapter and Pubsub
type Redis interface {
	Adapter
	Pubsub
}

// NewRedis generates Adapter with go-redis
func NewRedis(ring *redis.Ring) Redis {
	return &rds{
		ring:     ring,
		messChan: make(chan Message),
	}
}

type rds struct {
	ring       *redis.Ring
	subscriber *redis.PubSub

	subOnce   sync.Once
	closeOnce sync.Once
	messChan  chan Message
}

func (r *rds) MSet(
	ctx context.Context, keyVals map[string][]byte, ttl time.Duration, options ...MSetOptions,
) error {
	if len(keyVals) == 0 {
		return nil
	}

	_, err := r.ring.Pipelined(ctx, func(pipe redis.Pipeliner) error {
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

func (r *rds) MGet(ctx context.Context, keys []string) ([]Value, error) {
	vals, err := r.ring.MGet(ctx, keys...).Result()
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

func (r *rds) Del(ctx context.Context, keys ...string) error {
	_, err := r.ring.Del(ctx, keys...).Result()

	return err
}

type rdsMessage struct {
	topic   string
	content string
}

func (m *rdsMessage) Topic() string {
	return m.topic
}

func (m *rdsMessage) Content() []byte {
	return []byte(m.content)
}

func (r *rds) Pub(ctx context.Context, topic string, message []byte) error {
	return r.ring.Publish(ctx, topic, message).Err()
}

func (r *rds) Sub(ctx context.Context, topic ...string) <-chan Message {
	r.subOnce.Do(func() {
		r.subscriber = r.ring.Subscribe(ctx, topic...)

		go func() {
			for mess := range r.subscriber.Channel() {
				r.messChan <- &rdsMessage{
					topic:   mess.Channel,
					content: mess.Payload,
				}
			}

			close(r.messChan)
		}()
	})

	return r.messChan
}

func (r *rds) Close() {
	r.closeOnce.Do(func() {
		if r.subscriber != nil {
			r.subscriber.Close()
		}
	})
}
