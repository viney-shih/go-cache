package cache_test

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"github.com/viney-shih/go-cache"
)

type Person struct {
	FirstName string
	LastName  string
	Age       int
}

// Example_readThroughPattern will demo multiple cache layers and multiple
// prefix keys at the same time.
func Example_readThroughPattern() {
	tinyLfu := cache.NewTinyLFU(10000)
	rds := cache.NewRedis(redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{
			"server1": ":6379",
		},
	}))

	cacheF := cache.NewFactory(rds, tinyLfu)

	c := cacheF.NewCache([]cache.Setting{
		{
			Prefix: "teacher",
			CacheAttributes: map[cache.Type]cache.Attribute{
				cache.SharedCacheType: {TTL: time.Hour},
				cache.LocalCacheType:  {TTL: 10 * time.Minute},
			},
		},
		{
			Prefix: "student",
			CacheAttributes: map[cache.Type]cache.Attribute{
				cache.SharedCacheType: {TTL: time.Hour},
				cache.LocalCacheType:  {TTL: 10 * time.Minute},
			},
			MGetter: func(keys ...string) (interface{}, error) {
				// The MGetter is used to generate data when cache missed, and refill it to the cache automatically..
				// You can read from DB or other microservices.
				// Assume we read from MySQL according to the key "jacky" and get the value of
				// Person{FirstName: "jacky", LastName: "Lin", Age: 38}
				// HINT: remember to return as a slice, and the item order needs to consist with the keys in the parameters.
				if len(keys) == 1 && keys[0] == "jacky" {
					return []Person{{FirstName: "Jacky", LastName: "Lin", Age: 38}}, nil
				}

				return nil, fmt.Errorf("XD")
			},
		},
	})

	ctx := context.TODO()
	teacher := &Person{}
	if err := c.GetByFunc(ctx, "teacher", "jacky", teacher, func() (interface{}, error) {
		// The getter is used to generate data when cache missed, and refill it to the cache automatically..
		// You can read from DB or other microservices.
		// Assume we read from MySQL according to the key "jacky" and get the value of Object{Str: "value2", Num: 2}
		return Person{FirstName: "Jacky", LastName: "Wang", Age: 83}, nil
	}); err != nil {
		panic("not expected")
	}

	fmt.Println(teacher) // Output: {FirstName: "Jacky", LastName: "Wang", Age: 83}

	student := &Person{}
	if err := c.Get(ctx, "student", "jacky", student); err != nil {
		panic("not expected")
	}

	fmt.Println(student) // Output: {FirstName: "Jacky", LastName: "Lin", Age: 38}

	// Output:
	// &{Jacky Wang 83}
	// &{Jacky Lin 38}

}
