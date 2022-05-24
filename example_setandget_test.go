package cache_test

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/viney-shih/go-cache"
)

type Object struct {
	Str string
	Num int
}

func Example_setAndGetPattern() {
	tinyLfu := cache.NewTinyLFU(10000)
	rds := cache.NewRedis(redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{
			"server1": ":6379",
		},
	}))

	cacheF := cache.NewFactory(rds, tinyLfu)

	// We create a group of cache named "set-and-get".
	// It uses the shared cache only with TTL of ten seconds.
	c := cacheF.NewCache([]cache.Setting{
		{
			Prefix: "set-and-get",
			CacheAttributes: map[cache.Type]cache.Attribute{
				cache.SharedCacheType: {TTL: 10 * time.Second},
			},
		},
	})

	ctx := context.TODO()

	// set the cache
	obj := &Object{
		Str: "value1",
		Num: 1,
	}
	if err := c.Set(ctx, "set-and-get", "key", obj); err != nil {
		panic("not expected")
	}

	// read the cache
	container := &Object{}
	if err := c.Get(ctx, "set-and-get", "key", container); err != nil {
		panic("not expected")
	}
	fmt.Println(container) // Output: Object{ Str: "value1", Num: 1}

	// read the cache but failed
	if err := c.Get(ctx, "set-and-get", "no-such-key", container); err != nil {
		fmt.Println(err) //  Output: errors.New("cache key is missing")
	}

	// Output:
	// &{value1 1}
	// cache key is missing
}
