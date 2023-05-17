package cache

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/suite"
)

const (
	mockFactPfx = "fact-pfx"
	mockFactKey = "fact-key"
)

var (
	mockFactoryCTX = context.Background()
)

type factorySuite struct {
	suite.Suite

	factory *factory
	rds     *rds
	lfu     *tinyLFU
	ring    *redis.Ring
}

func (s *factorySuite) SetupSuite() {
	s.ring = redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{
			"server1": ":6379",
		},
	})
}

func (s *factorySuite) TearDownSuite() {}

func (s *factorySuite) SetupTest() {
	s.rds = NewRedis(s.ring).(*rds)
	s.lfu = NewTinyLFU(10000).(*tinyLFU)
	s.factory = NewFactory(s.rds, s.lfu).(*factory)
}

func (s *factorySuite) TearDownTest() {
	// prevent registering twice
	ClearPrefix()
	// flush redis
	_ = s.ring.ForEachShard(mockFactoryCTX, func(ctx context.Context, client *redis.Client) error {
		return client.FlushDB(ctx).Err()
	})

	s.factory.Close()
}

func TestFactorySuite(t *testing.T) {
	suite.Run(t, new(factorySuite))
}

func (s *factorySuite) TestNewFactoryWithOnlyMarshal() {
	defer func() {
		r := recover()
		s.Require().NotNil(r)
		s.Require().Equal(errors.New("both of Marshal and Unmarshal functions need to be specified"), r)
	}()
	NewFactory(s.rds, s.lfu, WithMarshalFunc(json.Marshal))
}

func (s *factorySuite) TestNewFactoryWithOnlyUnmarshal() {
	defer func() {
		r := recover()
		s.Require().NotNil(r)
		s.Require().Equal(errors.New("both of Marshal and Unmarshal functions need to be specified"), r)
	}()
	NewFactory(s.rds, s.lfu, WithUnmarshalFunc(json.Unmarshal))
}

func (s *factorySuite) TestNewFactoryWithBoth() {
	f := NewFactory(s.rds, s.lfu, WithMarshalFunc(xml.Marshal), WithUnmarshalFunc(xml.Unmarshal)).(*factory)
	s.Require().True(reflect.ValueOf(xml.Marshal).Pointer() == reflect.ValueOf(f.marshal).Pointer())
	s.Require().True(reflect.ValueOf(xml.Unmarshal).Pointer() == reflect.ValueOf(f.unmarshal).Pointer())
}

func (s *factorySuite) TestNewFactoryWithCacheHitAndMiss() {
	hitCount := 0
	missCount := 0

	// Due to use share cache only, init factory with NewEmpty()
	f := NewFactory(s.rds, NewEmpty(),
		OnCacheHitFunc(func(ctx context.Context, prefix, key string, count int) {
			s.Require().Equal(mockFactPfx, prefix)
			s.Require().Equal(mockFactKey, key)
			hitCount += count
		}),
		OnCacheMissFunc(func(ctx context.Context, prefix, key string, count int) {
			s.Require().Equal(mockFactPfx, prefix)
			s.Require().Equal(mockFactKey, key)
			missCount += count
		}),
	)

	var ret int
	var stage string
	c := f.NewCache([]Setting{
		{
			Prefix:          mockFactPfx,
			CacheAttributes: map[Type]Attribute{SharedCacheType: {time.Hour}},
		},
	})

	stage = "before"
	s.Require().Equal(0, hitCount, stage)
	s.Require().Equal(0, missCount, stage)

	stage = "get and miss"
	s.Require().Equal(ErrCacheMiss, c.Get(mockFactoryCTX, mockFactPfx, mockFactKey, &ret))
	s.Require().Equal(0, ret, stage)
	s.Require().Equal(0, hitCount, stage)
	s.Require().Equal(1, missCount, stage)

	stage = "set and get"
	s.Require().NoError(c.Set(mockFactoryCTX, mockFactPfx, mockFactKey, 100))
	s.Require().NoError(c.Get(mockFactoryCTX, mockFactPfx, mockFactKey, &ret))
	s.Require().Equal(100, ret, stage)
	s.Require().Equal(1, hitCount, stage)
	s.Require().Equal(1, missCount, stage)
}

func (s *factorySuite) TestNewFactoryWithCostAddAndEvict() {
	costAdd := 0
	costEvict := 0

	f := NewFactory(s.rds, s.lfu,
		OnLocalCacheCostAddFunc(func(ctx context.Context, prefix, key string, cost int) {
			s.Require().Equal(mockFactPfx, prefix)
			s.Require().Equal(mockFactKey, key)
			costAdd += cost
		}),
		OnLocalCacheCostEvictFunc(func(ctx context.Context, prefix, key string, cost int) {
			s.Require().Equal(mockFactPfx, prefix)
			s.Require().Equal(mockFactKey, key)
			costEvict += cost
		}),
	)

	//var ret int
	var stage string
	var bs []byte
	var err error
	c := f.NewCache([]Setting{
		{
			Prefix: mockFactPfx,
			CacheAttributes: map[Type]Attribute{
				SharedCacheType: {time.Hour},
				LocalCacheType:  {10 * time.Second},
			},
			MarshalFunc:   json.Marshal,
			UnmarshalFunc: json.Unmarshal,
		},
	})

	stage = "before"
	s.Require().Equal(0, costAdd, stage)
	s.Require().Equal(0, costEvict, stage)

	stage = "set"
	s.Require().NoError(c.Set(mockFactoryCTX, mockFactPfx, mockFactKey, 100))
	bs, err = json.Marshal(100)
	s.Require().NoError(err, stage)
	s.Require().Equal(len(bs), costAdd, stage)
	s.Require().Equal(0, costEvict, stage)

	stage = "del"
	s.Require().NoError(c.Del(mockFactoryCTX, mockFactPfx, mockFactKey))
	s.Require().Equal(len(bs), costAdd, stage)
	s.Require().Equal(len(bs), costEvict, stage)
}

func (s *factorySuite) TestNewCacheWithoutCacheType() {
	defer func() {
		r := recover()
		s.Require().NotNil(r)
		s.Require().Equal(errors.New("no cache type indicated"), r)
	}()
	s.factory.NewCache([]Setting{{Prefix: "noCacheType"}})
}

func (s *factorySuite) TestNewCacheWithEmptyPrefix() {
	defer func() {
		r := recover()
		s.Require().NotNil(r)
		s.Require().Equal(errors.New("not allowed empty prefix"), r)
	}()
	s.factory.NewCache([]Setting{{Prefix: ""}})
}

func (s *factorySuite) TestNewCacheWithDuplicatedPrefix() {
	defer func() {
		r := recover()
		s.Require().NotNil(r)
		s.Require().Equal(errors.New("duplicated prefix"), r)
	}()
	s.factory.NewCache([]Setting{
		{
			Prefix:          "exist",
			CacheAttributes: map[Type]Attribute{SharedCacheType: {time.Hour}},
		},
		{
			Prefix:          "exist",
			CacheAttributes: map[Type]Attribute{SharedCacheType: {time.Second}},
		},
	})
}

func (s *factorySuite) TestNewCacheWithOnlyMarshal() {
	defer func() {
		r := recover()
		s.Require().NotNil(r)
		s.Require().Equal(errors.New("both of Marshal and Unmarshal functions need to be specified"), r)
	}()

	s.factory.NewCache([]Setting{
		{
			Prefix:          "OnlyMarshal",
			CacheAttributes: map[Type]Attribute{SharedCacheType: {time.Hour}},
			MarshalFunc:     json.Marshal,
		},
	})
}

func (s *factorySuite) TestNewCacheWithOnlyUnmarshal() {
	defer func() {
		r := recover()
		s.Require().NotNil(r)
		s.Require().Equal(errors.New("both of Marshal and Unmarshal functions need to be specified"), r)
	}()

	s.factory.NewCache([]Setting{
		{
			Prefix:          "OnlyMarshal",
			CacheAttributes: map[Type]Attribute{SharedCacheType: {time.Hour}},
			UnmarshalFunc:   json.Unmarshal,
		},
	})
}
