package cache_test

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/viney-shih/go-cache"
)

func ExampleCache_GetByFunc() {
	tinyLfu := cache.NewTinyLFU(10000)
	rds := cache.NewRedis(redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{
			"server1": ":6379",
		},
	}))

	cacheF := cache.NewFactory(rds, tinyLfu)

	// We create a group of cache named "get-by-func".
	// It uses the local cache only with TTL of ten minutes.
	c := cacheF.NewCache([]cache.Setting{
		{
			Prefix: "get-by-func",
			CacheAttributes: map[cache.Type]cache.Attribute{
				cache.LocalCacheType: {TTL: 10 * time.Minute},
			},
		},
	})

	ctx := context.TODO()
	container2 := &Object{}
	if err := c.GetByFunc(ctx, "get-by-func", "key2", container2, func() (interface{}, error) {
		// The getter is used to generate data when cache missed, and refill the cache automatically..
		// You can read from DB or other microservices.
		// Assume we read from MySQL according to the key "key2" and get the value of Object{Str: "value2", Num: 2}
		return Object{Str: "value2", Num: 2}, nil
	}); err != nil {
		panic("not expected")
	}

	fmt.Println(container2) // Object{ Str: "value2", Num: 2}

	// Output:
	// &{value2 2}
}

func ExampleFactory_NewCache_mGetter() {
	tinyLfu := cache.NewTinyLFU(10000)
	rds := cache.NewRedis(redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{
			"server1": ":6379",
		},
	}))

	cacheF := cache.NewFactory(rds, tinyLfu)

	// We create a group of cache named "mgetter".
	// It uses both shared and local caches with separated TTL of one hour and ten minutes.
	c := cacheF.NewCache([]cache.Setting{
		{
			Prefix: "mgetter",
			CacheAttributes: map[cache.Type]cache.Attribute{
				cache.SharedCacheType: {TTL: time.Hour},
				cache.LocalCacheType:  {TTL: 10 * time.Minute},
			},
			MGetter: func(ctx context.Context, keys ...string) (interface{}, error) {
				// The MGetter is used to generate data when cache missed, and refill the cache automatically..
				// You can read from DB or other microservices.
				// Assume we read from MySQL according to the key "key3" and get the value of Object{Str: "value3", Num: 3}
				// HINT: remember to return as a slice, and the item order needs to consist with the keys in the parameters.
				return []Object{{Str: "value3", Num: 3}}, nil
			},
		},
	})

	ctx := context.TODO()
	container3 := &Object{}
	if err := c.Get(ctx, "mgetter", "key3", container3); err != nil {
		panic("not expected")
	}

	fmt.Println(container3) // Object{ Str: "value3", Num: 3}

	// Output:
	// &{value3 3}
}
