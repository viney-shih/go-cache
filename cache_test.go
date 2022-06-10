package cache

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/suite"
	"github.com/vmihailenco/go-tinylfu"
)

const (
	mockString = "mock-string"
)

var (
	mockCacheCTX = context.Background()
)

type cacheSuite struct {
	suite.Suite

	factory *factory
	rds     *rds
	lfu     *tinyLFU
	ring    *redis.Ring
}

func (s *cacheSuite) SetupSuite() {
	s.ring = redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{
			"server1": ":6379",
		},
	})
}

func (s *cacheSuite) TearDownSuite() {}

func (s *cacheSuite) SetupTest() {
	s.rds = NewRedis(s.ring).(*rds)
	s.lfu = NewTinyLFU(10000).(*tinyLFU)
	s.factory = NewFactory(s.rds, s.lfu).(*factory)
}

func (s *cacheSuite) TearDownTest() {
	// prevent registering twice
	ClearPrefix()
	// flush redis
	_ = s.ring.ForEachShard(mockCacheCTX, func(ctx context.Context, client *redis.Client) error {
		return client.FlushDB(ctx).Err()
	})

	s.factory.Close()
}

func TestCacheSuite(t *testing.T) {
	suite.Run(t, new(cacheSuite))
}

func (s *cacheSuite) TestMSet() {
	tests := []struct {
		Desc      string
		Settings  []Setting
		Prefix    string
		KeyValues map[string]interface{}
		ExpError  map[string]error
		CheckFunc map[string]func(string)
	}{
		{
			Desc: "prefix not registered",
			Settings: []Setting{{
				Prefix: "registered", CacheAttributes: map[Type]Attribute{SharedCacheType: {TTL: time.Hour}},
			}},
			Prefix: "not-registered",
			ExpError: map[string]error{
				"not-registered": ErrPfxNotRegistered,
			},
		},
		{
			Desc: "normal MSet",
			Settings: []Setting{
				{
					Prefix: "mixed",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
						LocalCacheType:  {TTL: time.Hour},
					},
				},
				{
					Prefix: "redis",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
					},
				},
				{
					Prefix: "local",
					CacheAttributes: map[Type]Attribute{
						LocalCacheType: {TTL: time.Hour},
					},
				},
			},
			KeyValues: map[string]interface{}{
				"keyS": mockString,
				"keyI": 80,
			},
			ExpError: map[string]error{
				"mixed": nil,
				"redis": nil,
				"local": nil,
			},
			CheckFunc: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKeyS := getCacheKey("mixed", "keyS")
					cacheKeyI := getCacheKey("mixed", "keyI")
					expSB, _ := json.Marshal(mockString)
					expIB, _ := json.Marshal(80)

					b, exist := s.lfu.lfu.Get(cacheKeyS)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(expSB, b, desc, "mixed")
					b, exist = s.lfu.lfu.Get(cacheKeyI)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(expIB, b, desc, "mixed")

					b, err := s.ring.Get(mockCacheCTX, cacheKeyS).Bytes()
					s.Require().NoError(err, desc, "mixed")
					s.Require().Equal(expSB, b, desc, "mixed")
					b, err = s.ring.Get(mockCacheCTX, cacheKeyI).Bytes()
					s.Require().NoError(err, desc, "mixed")
					s.Require().Equal(expIB, b, desc, "mixed")
				},
				"redis": func(desc string) {
					cacheKeyS := getCacheKey("redis", "keyS")
					cacheKeyI := getCacheKey("redis", "keyI")
					expSB, _ := json.Marshal(mockString)
					expIB, _ := json.Marshal(80)

					_, exist := s.lfu.lfu.Get(cacheKeyS)
					s.Require().False(exist, desc, "redis")
					_, exist = s.lfu.lfu.Get(cacheKeyI)
					s.Require().False(exist, desc, "redis")

					b, err := s.ring.Get(mockCacheCTX, cacheKeyS).Bytes()
					s.Require().NoError(err, desc, "redis")
					s.Require().Equal(expSB, b, desc, "redis")
					b, err = s.ring.Get(mockCacheCTX, cacheKeyI).Bytes()
					s.Require().NoError(err, desc, "redis")
					s.Require().Equal(expIB, b, desc, "redis")
				},
				"local": func(desc string) {
					cacheKeyS := getCacheKey("local", "keyS")
					cacheKeyI := getCacheKey("local", "keyI")
					expSB, _ := json.Marshal(mockString)
					expIB, _ := json.Marshal(80)

					b, exist := s.lfu.lfu.Get(cacheKeyS)
					s.Require().True(exist, desc, "local")
					s.Require().Equal(expSB, b, desc, "local")
					b, exist = s.lfu.lfu.Get(cacheKeyI)
					s.Require().True(exist, desc, "local")
					s.Require().Equal(expIB, b, desc, "local")

					_, err := s.ring.Get(mockCacheCTX, cacheKeyS).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "local")
					_, err = s.ring.Get(mockCacheCTX, cacheKeyI).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "local")
				},
			},
		},
	}

	for _, t := range tests {
		c := s.factory.NewCache(t.Settings)

		for _, sett := range t.Settings {
			pfx := sett.Prefix
			if t.Prefix != "" {
				pfx = t.Prefix
			}
			err := c.MSet(mockCacheCTX, pfx, t.KeyValues)
			s.Require().Equal(t.ExpError[pfx], err, t.Desc)

			if t.CheckFunc[pfx] != nil {
				t.CheckFunc[pfx](t.Desc)
			}

			s.TearDownTest()
		}
	}
}

func (s *cacheSuite) TestSet() {
	tests := []struct {
		Desc      string
		Settings  []Setting
		Prefix    string
		Key       string
		Value     interface{}
		ExpError  map[string]error
		CheckFunc map[string]func(string)
	}{
		{
			Desc: "prefix not registered",
			Settings: []Setting{{
				Prefix: "registered", CacheAttributes: map[Type]Attribute{SharedCacheType: {TTL: time.Hour}},
			}},
			Prefix: "not-registered",
			ExpError: map[string]error{
				"not-registered": ErrPfxNotRegistered,
			},
		},
		{
			Desc: "normal Set",
			Settings: []Setting{
				{
					Prefix: "mixed",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
						LocalCacheType:  {TTL: time.Hour},
					},
				},
				{
					Prefix: "redis",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
					},
				},
				{
					Prefix: "local",
					CacheAttributes: map[Type]Attribute{
						LocalCacheType: {TTL: time.Hour},
					},
				},
			},
			Key:   "key",
			Value: float32(13.38),
			ExpError: map[string]error{
				"mixed": nil,
				"redis": nil,
				"local": nil,
			},
			CheckFunc: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKey := getCacheKey("mixed", "key")
					expB, _ := json.Marshal(float32(13.38))

					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")

					b, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().NoError(err, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")
				},
				"redis": func(desc string) {
					cacheKey := getCacheKey("redis", "key")
					expB, _ := json.Marshal(float32(13.38))

					_, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().False(exist, desc, "redis")

					b, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().NoError(err, desc, "redis")
					s.Require().Equal(expB, b, desc, "redis")
				},
				"local": func(desc string) {
					cacheKey := getCacheKey("local", "key")
					expB, _ := json.Marshal(float32(13.38))

					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "local")
					s.Require().Equal(expB, b, desc, "local")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "local")
				},
			},
		},
	}

	for _, t := range tests {
		c := s.factory.NewCache(t.Settings)

		for _, sett := range t.Settings {
			pfx := sett.Prefix
			if t.Prefix != "" {
				pfx = t.Prefix
			}
			err := c.Set(mockCacheCTX, pfx, t.Key, t.Value)
			s.Require().Equal(t.ExpError[pfx], err, t.Desc)

			if t.CheckFunc[pfx] != nil {
				t.CheckFunc[pfx](t.Desc)
			}

			s.TearDownTest()
		}
	}
}

func (s *cacheSuite) TestDel() {
	tests := []struct {
		Desc      string
		Settings  []Setting
		Prefix    string
		SetupTest map[string]func(string)
		Keys      []string
		ExpError  map[string]error
		CheckFunc map[string]func(string)
	}{
		{
			Desc: "prefix not registered",
			Settings: []Setting{{
				Prefix: "registered", CacheAttributes: map[Type]Attribute{SharedCacheType: {TTL: time.Hour}},
			}},
			Prefix: "not-registered",
			ExpError: map[string]error{
				"not-registered": ErrPfxNotRegistered,
			},
		},
		{
			Desc: "normal Del",
			Settings: []Setting{
				{
					Prefix: "mixed",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
						LocalCacheType:  {TTL: time.Hour},
					},
				},
				{
					Prefix: "redis",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
					},
				},
				{
					Prefix: "local",
					CacheAttributes: map[Type]Attribute{
						LocalCacheType: {TTL: time.Hour},
					},
				},
			},
			SetupTest: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKey := getCacheKey("mixed", "key")
					expB, _ := json.Marshal(mockString)

					s.lfu.lfu.Set(&tinylfu.Item{
						Key:      cacheKey,
						Value:    expB,
						ExpireAt: time.Now().Add(time.Hour),
					})
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")

					s.Require().NoError(s.ring.Set(mockCacheCTX, cacheKey, expB, time.Hour).Err(), desc)
					b, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().NoError(err, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")
				},
				"redis": func(desc string) {
					cacheKey := getCacheKey("redis", "key")
					expB, _ := json.Marshal(mockString)

					s.lfu.lfu.Set(&tinylfu.Item{
						Key:      cacheKey,
						Value:    expB,
						ExpireAt: time.Now().Add(time.Hour),
					})
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "redis")
					s.Require().Equal(expB, b, desc, "redis")

					s.Require().NoError(s.ring.Set(mockCacheCTX, cacheKey, expB, time.Hour).Err(), desc)
					b, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().NoError(err, desc, "redis")
					s.Require().Equal(expB, b, desc, "redis")
				},
				"local": func(desc string) {
					cacheKey := getCacheKey("local", "key")
					expB, _ := json.Marshal(mockString)

					s.lfu.lfu.Set(&tinylfu.Item{
						Key:      cacheKey,
						Value:    expB,
						ExpireAt: time.Now().Add(time.Hour),
					})
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "local")
					s.Require().Equal(expB, b, desc, "local")

					s.Require().NoError(s.ring.Set(mockCacheCTX, cacheKey, expB, time.Hour).Err(), desc)
					b, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().NoError(err, desc, "local")
					s.Require().Equal(expB, b, desc, "local")
				},
			},
			Keys: []string{"key", "not-existed"},
			ExpError: map[string]error{
				"mixed": nil,
				"redis": nil,
				"local": nil,
			},
			CheckFunc: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKey := getCacheKey("mixed", "key")

					_, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().False(exist, desc, "mixed")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "mixed")
				},
				"redis": func(desc string) {
					cacheKey := getCacheKey("redis", "key")
					expB, _ := json.Marshal(mockString)

					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "redis")
					s.Require().Equal(expB, b, desc, "redis")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "redis")
				},
				"local": func(desc string) {
					cacheKey := getCacheKey("local", "key")
					expB, _ := json.Marshal(mockString)

					_, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().False(exist, desc, "local")

					b, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().NoError(err, desc, "local")
					s.Require().Equal(expB, b, desc, "local")
				},
			},
		},
	}

	for _, t := range tests {
		c := s.factory.NewCache(t.Settings)

		for _, sett := range t.Settings {
			pfx := sett.Prefix
			if t.Prefix != "" {
				pfx = t.Prefix
			}

			if t.SetupTest[pfx] != nil {
				t.SetupTest[pfx](t.Desc)
			}

			err := c.Del(mockCacheCTX, pfx, t.Keys...)
			s.Require().Equal(t.ExpError[pfx], err, t.Desc)

			if t.CheckFunc[pfx] != nil {
				t.CheckFunc[pfx](t.Desc)
			}

			s.TearDownTest()
		}
	}
}

type resultPair struct {
	err   error
	value interface{}
}

func (s *cacheSuite) TestMGet() {
	tests := []struct {
		Desc           string
		Settings       []Setting
		Prefix         string
		SetupTest      map[string]func(string)
		Keys           []string
		ExpError       map[string]error
		ExpResultLen   map[string]int
		ExpResultValue map[string][]resultPair
		CheckFunc      map[string]func(string)
	}{
		{
			Desc: "prefix not registered",
			Settings: []Setting{{
				Prefix: "registered", CacheAttributes: map[Type]Attribute{SharedCacheType: {TTL: time.Hour}},
			}},
			Prefix: "not-registered",
			ExpError: map[string]error{
				"not-registered": ErrPfxNotRegistered,
			},
		},
		{
			Desc: "MGet in local",
			Settings: []Setting{
				{
					Prefix: "mixed",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
						LocalCacheType:  {TTL: time.Hour},
					},
				},
				{
					Prefix: "redis",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
					},
				},
				{
					Prefix: "local",
					CacheAttributes: map[Type]Attribute{
						LocalCacheType: {TTL: time.Hour},
					},
				},
			},
			SetupTest: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKey := getCacheKey("mixed", "key")
					expB, _ := json.Marshal(mockString)

					s.lfu.lfu.Set(&tinylfu.Item{
						Key:      cacheKey,
						Value:    expB,
						ExpireAt: time.Now().Add(time.Hour),
					})
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "mixed")
				},
				"redis": func(desc string) {
					cacheKey := getCacheKey("redis", "key")
					expB, _ := json.Marshal(mockString)

					s.lfu.lfu.Set(&tinylfu.Item{
						Key:      cacheKey,
						Value:    expB,
						ExpireAt: time.Now().Add(time.Hour),
					})
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "redis")
					s.Require().Equal(expB, b, desc, "redis")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "redis")
				},
				"local": func(desc string) {
					cacheKey := getCacheKey("local", "key")
					expB, _ := json.Marshal(mockString)

					s.lfu.lfu.Set(&tinylfu.Item{
						Key:      cacheKey,
						Value:    expB,
						ExpireAt: time.Now().Add(time.Hour),
					})
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "local")
					s.Require().Equal(expB, b, desc, "local")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "local")
				},
			},
			Keys: []string{"key", "not-existed"},
			ExpError: map[string]error{
				"mixed": nil,
				"redis": nil,
				"local": nil,
			},
			ExpResultLen: map[string]int{
				"mixed": 2,
				"redis": 2,
				"local": 2,
			},
			ExpResultValue: map[string][]resultPair{
				"mixed": {{value: mockString, err: nil}, {value: "", err: ErrCacheMiss}},
				"redis": {{value: "", err: ErrCacheMiss}, {value: "", err: ErrCacheMiss}},
				"local": {{value: mockString, err: nil}, {value: "", err: ErrCacheMiss}},
			},
			CheckFunc: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKey := getCacheKey("mixed", "key")
					expB, _ := json.Marshal(mockString)

					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "mixed")
				},
				"redis": func(desc string) {
					cacheKey := getCacheKey("redis", "key")
					expB, _ := json.Marshal(mockString)

					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "redis")
					s.Require().Equal(expB, b, desc, "redis")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "redis")
				},
				"local": func(desc string) {
					cacheKey := getCacheKey("local", "key")
					expB, _ := json.Marshal(mockString)

					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "local")
					s.Require().Equal(expB, b, desc, "local")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "local")
				},
			},
		},
		{
			Desc: "MGet in redis",
			Settings: []Setting{
				{
					Prefix: "mixed",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
						LocalCacheType:  {TTL: time.Hour},
					},
				},
				{
					Prefix: "redis",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
					},
				},
				{
					Prefix: "local",
					CacheAttributes: map[Type]Attribute{
						LocalCacheType: {TTL: time.Hour},
					},
				},
			},
			SetupTest: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKey := getCacheKey("mixed", "key")
					expB, _ := json.Marshal(mockString)

					_, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().False(exist, desc, "mixed")

					s.Require().NoError(s.ring.Set(mockCacheCTX, cacheKey, expB, time.Hour).Err(), desc)
					b, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().NoError(err, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")
				},
				"redis": func(desc string) {
					cacheKey := getCacheKey("redis", "key")
					expB, _ := json.Marshal(mockString)

					_, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().False(exist, desc, "redis")

					s.Require().NoError(s.ring.Set(mockCacheCTX, cacheKey, expB, time.Hour).Err(), desc)
					b, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().NoError(err, desc, "redis")
					s.Require().Equal(expB, b, desc, "redis")
				},
				"local": func(desc string) {
					cacheKey := getCacheKey("local", "key")
					expB, _ := json.Marshal(mockString)

					_, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().False(exist, desc, "local")

					s.Require().NoError(s.ring.Set(mockCacheCTX, cacheKey, expB, time.Hour).Err(), desc)
					b, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().NoError(err, desc, "local")
					s.Require().Equal(expB, b, desc, "local")
				},
			},
			Keys: []string{"key", "not-existed"},
			ExpError: map[string]error{
				"mixed": nil,
				"redis": nil,
				"local": nil,
			},
			ExpResultLen: map[string]int{
				"mixed": 2,
				"redis": 2,
				"local": 2,
			},
			ExpResultValue: map[string][]resultPair{
				"mixed": {{value: mockString, err: nil}, {value: "", err: ErrCacheMiss}},
				"redis": {{value: mockString, err: nil}, {value: "", err: ErrCacheMiss}},
				"local": {{value: "", err: ErrCacheMiss}, {value: "", err: ErrCacheMiss}},
			},
			CheckFunc: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKey := getCacheKey("mixed", "key")
					expB, _ := json.Marshal(mockString)

					// refill the cache in local
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")

					b, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().NoError(err, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")
				},
				"redis": func(desc string) {
					cacheKey := getCacheKey("redis", "key")
					expB, _ := json.Marshal(mockString)

					_, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().False(exist, desc, "redis")

					b, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().NoError(err, desc, "redis")
					s.Require().Equal(expB, b, desc, "redis")
				},
				"local": func(desc string) {
					cacheKey := getCacheKey("local", "key")
					expB, _ := json.Marshal(mockString)

					_, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().False(exist, desc, "local")

					b, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().NoError(err, desc, "local")
					s.Require().Equal(expB, b, desc, "local")
				},
			},
		},
		{
			Desc: "MGet in local with MGetter",
			Settings: []Setting{
				{
					Prefix: "mixed",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
						LocalCacheType:  {TTL: time.Hour},
					},
					MGetter: func(keys ...string) (interface{}, error) {
						s.Require().Equal([]string{"not-existed"}, keys)
						return []string{"mgetter-existed"}, nil
					},
				},
				{
					Prefix: "redis",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
					},
					MGetter: func(keys ...string) (interface{}, error) {
						s.Require().Equal([]string{"key", "not-existed"}, keys)
						return []string{"mgetter-key", "mgetter-existed"}, nil
					},
				},
				{
					Prefix: "local",
					CacheAttributes: map[Type]Attribute{
						LocalCacheType: {TTL: time.Hour},
					},
					MGetter: func(keys ...string) (interface{}, error) {
						s.Require().Equal([]string{"not-existed"}, keys)
						return []string{"mgetter-existed"}, nil
					},
				},
			},
			SetupTest: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKey := getCacheKey("mixed", "key")
					expB, _ := json.Marshal(mockString)

					s.lfu.lfu.Set(&tinylfu.Item{
						Key:      cacheKey,
						Value:    expB,
						ExpireAt: time.Now().Add(time.Hour),
					})
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "mixed")
				},
				"redis": func(desc string) {
					cacheKey := getCacheKey("redis", "key")
					expB, _ := json.Marshal(mockString)

					s.lfu.lfu.Set(&tinylfu.Item{
						Key:      cacheKey,
						Value:    expB,
						ExpireAt: time.Now().Add(time.Hour),
					})
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "redis")
					s.Require().Equal(expB, b, desc, "redis")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "redis")
				},
				"local": func(desc string) {
					cacheKey := getCacheKey("local", "key")
					expB, _ := json.Marshal(mockString)

					s.lfu.lfu.Set(&tinylfu.Item{
						Key:      cacheKey,
						Value:    expB,
						ExpireAt: time.Now().Add(time.Hour),
					})
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "local")
					s.Require().Equal(expB, b, desc, "local")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "local")
				},
			},
			Keys: []string{"key", "not-existed", "key"}, // duplicated key
			ExpError: map[string]error{
				"mixed": nil,
				"redis": nil,
				"local": nil,
			},
			ExpResultLen: map[string]int{
				"mixed": 3,
				"redis": 3,
				"local": 3,
			},
			ExpResultValue: map[string][]resultPair{
				"mixed": {{value: mockString, err: nil}, {value: "mgetter-existed", err: nil}, {value: mockString, err: nil}},
				"redis": {{value: "mgetter-key", err: nil}, {value: "mgetter-existed", err: nil}, {value: "mgetter-key", err: nil}},
				"local": {{value: mockString, err: nil}, {value: "mgetter-existed", err: nil}, {value: mockString, err: nil}},
			},
			CheckFunc: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKey := getCacheKey("mixed", "key")
					expB, _ := json.Marshal(mockString)

					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "mixed")

					// check refilled in cache
					notExistKey := getCacheKey("mixed", "not-existed")
					notExistB, _ := json.Marshal("mgetter-existed")

					b, exist = s.lfu.lfu.Get(notExistKey)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(notExistB, b, desc, "mixed")

					b, err = s.ring.Get(mockCacheCTX, notExistKey).Bytes()
					s.Require().NoError(err, desc, "mixed")
					s.Require().Equal(notExistB, b, desc, "mixed")
				},
				"redis": func(desc string) {
					cacheKey := getCacheKey("redis", "key")
					expB, _ := json.Marshal(mockString)
					mGetterB, _ := json.Marshal("mgetter-key")

					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "redis")
					s.Require().Equal(expB, b, desc, "redis")

					b, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().NoError(err, desc, "redis")
					s.Require().Equal(mGetterB, b, desc, "redis")

					// check refilled in cache
					notExistKey := getCacheKey("redis", "not-existed")
					notExistB, _ := json.Marshal("mgetter-existed")

					_, exist = s.lfu.lfu.Get(notExistKey)
					s.Require().False(exist, desc, "redis")

					b, err = s.ring.Get(mockCacheCTX, notExistKey).Bytes()
					s.Require().NoError(err, desc, "redis")
					s.Require().Equal(notExistB, b, desc, "redis")
				},
				"local": func(desc string) {
					cacheKey := getCacheKey("local", "key")
					expB, _ := json.Marshal(mockString)

					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "local")
					s.Require().Equal(expB, b, desc, "local")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "local")

					// check refilled in cache
					notExistKey := getCacheKey("local", "not-existed")
					notExistB, _ := json.Marshal("mgetter-existed")

					b, exist = s.lfu.lfu.Get(notExistKey)
					s.Require().True(exist, desc, "local")
					s.Require().Equal(notExistB, b, desc, "local")

					_, err = s.ring.Get(mockCacheCTX, notExistKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "local")
				},
			},
		},
	}

	for _, t := range tests {
		c := s.factory.NewCache(t.Settings).(*cache)

		for _, sett := range t.Settings {
			pfx := sett.Prefix
			if t.Prefix != "" {
				pfx = t.Prefix
			}

			if t.SetupTest[pfx] != nil {
				t.SetupTest[pfx](t.Desc)
			}

			r, err := c.MGet(mockCacheCTX, pfx, t.Keys...)
			s.Require().Equal(t.ExpError[pfx], err, t.Desc)
			if err == nil {
				s.Require().Equal(t.ExpResultLen[pfx], r.Len())

				if r.Len() != 0 {
					vs := make([]string, r.Len())
					rs := make([]resultPair, r.Len())
					for i := 0; i < r.Len(); i++ {
						err := r.Get(mockCacheCTX, i, &vs[i])
						rs[i].err = err
						rs[i].value = vs[i]
					}
					s.Require().Equal(t.ExpResultValue[pfx], rs, t.Desc)
				}
			}

			if t.CheckFunc[pfx] != nil {
				t.CheckFunc[pfx](t.Desc)
			}

			// clean up the cache
			s.TearDownTest()
			s.factory.localCache.Del(mockCacheCTX, getCacheKeys(sett.Prefix, t.Keys)...)
		}
	}
}

func (s *cacheSuite) TestGet() {
	tests := []struct {
		Desc      string
		Settings  []Setting
		Prefix    string
		SetupTest map[string]func(string)
		Key       string
		ExpError  map[string]error
		ExpResult map[string]interface{}
	}{
		{
			Desc: "prefix not registered",
			Settings: []Setting{{
				Prefix: "registered", CacheAttributes: map[Type]Attribute{SharedCacheType: {TTL: time.Hour}},
			}},
			Prefix: "not-registered",
			ExpError: map[string]error{
				"not-registered": ErrPfxNotRegistered,
			},
		},
		{
			Desc: "Get hit",
			Settings: []Setting{
				{
					Prefix: "mixed",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
						LocalCacheType:  {TTL: time.Hour},
					},
				},
			},
			SetupTest: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKey := getCacheKey("mixed", "key")
					expB, _ := json.Marshal(mockString)

					s.lfu.lfu.Set(&tinylfu.Item{
						Key:      cacheKey,
						Value:    expB,
						ExpireAt: time.Now().Add(time.Hour),
					})
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "mixed")
				},
			},
			Key: "key",
			ExpError: map[string]error{
				"mixed": nil,
			},
			ExpResult: map[string]interface{}{
				"mixed": mockString,
			},
		},
		{
			Desc: "Get miss",
			Settings: []Setting{
				{
					Prefix: "mixed",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
						LocalCacheType:  {TTL: time.Hour},
					},
				},
			},
			SetupTest: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKey := getCacheKey("mixed", "key")
					expB, _ := json.Marshal(mockString)

					s.lfu.lfu.Set(&tinylfu.Item{
						Key:      cacheKey,
						Value:    expB,
						ExpireAt: time.Now().Add(time.Hour),
					})
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "mixed")
				},
			},
			Key: "not-existed",
			ExpError: map[string]error{
				"mixed": ErrCacheMiss,
			},
		},
		{
			Desc: "Get miss but refill again",
			Settings: []Setting{
				{
					Prefix: "mixed",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
						LocalCacheType:  {TTL: time.Hour},
					},
					MGetter: func(keys ...string) (interface{}, error) {
						s.Require().Equal([]string{"not-existed"}, keys)
						return []string{"mgetter-existed"}, nil
					},
				},
			},
			SetupTest: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKey := getCacheKey("mixed", "key")
					expB, _ := json.Marshal(mockString)

					s.lfu.lfu.Set(&tinylfu.Item{
						Key:      cacheKey,
						Value:    expB,
						ExpireAt: time.Now().Add(time.Hour),
					})
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "mixed")
				},
			},
			Key: "not-existed",
			ExpError: map[string]error{
				"mixed": nil,
			},
			ExpResult: map[string]interface{}{
				"mixed": "mgetter-existed",
			},
		},
		{
			Desc: "Get miss but refill failed",
			Settings: []Setting{
				{
					Prefix: "mixed",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
						LocalCacheType:  {TTL: time.Hour},
					},
					MGetter: func(keys ...string) (interface{}, error) {
						s.Require().Equal([]string{"XD"}, keys)
						return nil, errors.New("XD")
					},
				},
			},
			SetupTest: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKey := getCacheKey("mixed", "key")
					expB, _ := json.Marshal(mockString)

					s.lfu.lfu.Set(&tinylfu.Item{
						Key:      cacheKey,
						Value:    expB,
						ExpireAt: time.Now().Add(time.Hour),
					})
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "mixed")
				},
			},
			Key: "XD",
			ExpError: map[string]error{
				"mixed": errors.New("XD"),
			},
		},
	}

	for _, t := range tests {
		c := s.factory.NewCache(t.Settings).(*cache)

		for _, sett := range t.Settings {
			pfx := sett.Prefix
			if t.Prefix != "" {
				pfx = t.Prefix
			}

			if t.SetupTest[pfx] != nil {
				t.SetupTest[pfx](t.Desc)
			}

			result := ""
			err := c.Get(mockCacheCTX, pfx, t.Key, &result)
			s.Require().Equal(t.ExpError[pfx], err, t.Desc)
			if err == nil {
				s.Require().Equal(t.ExpResult[pfx], result, t.Desc)
			}

			s.TearDownTest()
		}
	}
}

func (s *cacheSuite) TestGetByFunc() {
	tests := []struct {
		Desc      string
		Settings  []Setting
		Prefix    string
		SetupTest map[string]func(string)
		Key       string
		Getter    map[string]func() (interface{}, error)
		ExpError  map[string]error
		ExpResult map[string]interface{}
		CheckFunc map[string]func(string)
	}{
		{
			Desc: "prefix not registered",
			Settings: []Setting{{
				Prefix: "registered", CacheAttributes: map[Type]Attribute{SharedCacheType: {TTL: time.Hour}},
			}},
			Prefix: "not-registered",
			ExpError: map[string]error{
				"not-registered": ErrPfxNotRegistered,
			},
		},
		{
			Desc: "Get hit",
			Settings: []Setting{
				{
					Prefix: "mixed",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
						LocalCacheType:  {TTL: time.Hour},
					},
				},
			},
			SetupTest: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKey := getCacheKey("mixed", "key")
					expB, _ := json.Marshal(mockString)

					s.lfu.lfu.Set(&tinylfu.Item{
						Key:      cacheKey,
						Value:    expB,
						ExpireAt: time.Now().Add(time.Hour),
					})
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "mixed")
				},
			},
			Key: "key",
			ExpError: map[string]error{
				"mixed": nil,
			},
			ExpResult: map[string]interface{}{
				"mixed": mockString,
			},
		},
		{
			Desc: "Get miss but refill again",
			Settings: []Setting{
				{
					Prefix: "mixed",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
						LocalCacheType:  {TTL: time.Hour},
					},
				},
			},
			SetupTest: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKey := getCacheKey("mixed", "key")
					expB, _ := json.Marshal(mockString)

					s.lfu.lfu.Set(&tinylfu.Item{
						Key:      cacheKey,
						Value:    expB,
						ExpireAt: time.Now().Add(time.Hour),
					})
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "mixed")
				},
			},
			Key: "not-existed",
			Getter: map[string]func() (interface{}, error){
				"mixed": func() (interface{}, error) {
					return "one-time-getter-existed", nil
				},
			},
			ExpError: map[string]error{
				"mixed": nil,
			},
			ExpResult: map[string]interface{}{
				"mixed": "one-time-getter-existed",
			},
			CheckFunc: map[string]func(desc string){
				"mixed": func(desc string) {
					// check refilled in cache
					notExistKey := getCacheKey("mixed", "not-existed")
					notExistB, _ := json.Marshal("one-time-getter-existed")

					b, exist := s.lfu.lfu.Get(notExistKey)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(notExistB, b, desc, "mixed")

					b, err := s.ring.Get(mockCacheCTX, notExistKey).Bytes()
					s.Require().NoError(err, desc, "mixed")
					s.Require().Equal(notExistB, b, desc, "mixed")
				},
			},
		},
		{
			Desc: "Get miss but refill failed",
			Settings: []Setting{
				{
					Prefix: "mixed",
					CacheAttributes: map[Type]Attribute{
						SharedCacheType: {TTL: time.Hour},
						LocalCacheType:  {TTL: time.Hour},
					},
				},
			},
			SetupTest: map[string]func(desc string){
				"mixed": func(desc string) {
					cacheKey := getCacheKey("mixed", "key")
					expB, _ := json.Marshal(mockString)

					s.lfu.lfu.Set(&tinylfu.Item{
						Key:      cacheKey,
						Value:    expB,
						ExpireAt: time.Now().Add(time.Hour),
					})
					b, exist := s.lfu.lfu.Get(cacheKey)
					s.Require().True(exist, desc, "mixed")
					s.Require().Equal(expB, b, desc, "mixed")

					_, err := s.ring.Get(mockCacheCTX, cacheKey).Bytes()
					s.Require().Equal(redis.Nil, err, desc, "mixed")
				},
			},
			Key: "XD",
			Getter: map[string]func() (interface{}, error){
				"mixed": func() (interface{}, error) {
					return nil, errors.New("XD")
				},
			},
			ExpError: map[string]error{
				"mixed": errors.New("XD"),
			},
		},
	}

	for _, t := range tests {
		c := s.factory.NewCache(t.Settings).(*cache)

		for _, sett := range t.Settings {
			pfx := sett.Prefix
			if t.Prefix != "" {
				pfx = t.Prefix
			}

			if t.SetupTest[pfx] != nil {
				t.SetupTest[pfx](t.Desc)
			}

			result := ""
			err := c.GetByFunc(mockCacheCTX, pfx, t.Key, &result, t.Getter[pfx])
			s.Require().Equal(t.ExpError[pfx], err, t.Desc)
			if err == nil {
				s.Require().Equal(t.ExpResult[pfx], result, t.Desc)
			}

			if t.CheckFunc[pfx] != nil {
				t.CheckFunc[pfx](t.Desc)
			}

			s.TearDownTest()
		}
	}
}
