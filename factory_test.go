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
}

func TestFactorySuite(t *testing.T) {
	suite.Run(t, new(factorySuite))
}

func (s *factorySuite) TestNewWithOnlyMarshal() {
	defer func() {
		r := recover()
		s.Require().NotNil(r)
		s.Require().Equal(errors.New("both of Marshal and Unmarshal functions need to be specified"), r)
	}()
	NewFactory(s.rds, s.lfu, WithMarshalFunc(json.Marshal))
}

func (s *factorySuite) TestNewWithOnlyUnmarshal() {
	defer func() {
		r := recover()
		s.Require().NotNil(r)
		s.Require().Equal(errors.New("both of Marshal and Unmarshal functions need to be specified"), r)
	}()
	NewFactory(s.rds, s.lfu, WithUnmarshalFunc(json.Unmarshal))
}

func (s *factorySuite) TestNewWithBoth() {
	ser := NewFactory(s.rds, s.lfu, WithMarshalFunc(xml.Marshal), WithUnmarshalFunc(xml.Unmarshal)).(*factory)
	s.Require().True(reflect.ValueOf(xml.Marshal).Pointer() == reflect.ValueOf(ser.marshal).Pointer())
	s.Require().True(reflect.ValueOf(xml.Unmarshal).Pointer() == reflect.ValueOf(ser.unmarshal).Pointer())
}

func (s *factorySuite) TestCreateWithoutCacheType() {
	defer func() {
		r := recover()
		s.Require().NotNil(r)
		s.Require().Equal(errors.New("no cache type indicated"), r)
	}()
	s.factory.NewCache([]Setting{{Prefix: "noCacheType"}})
}

func (s *factorySuite) TestCreateWithEmptyPrefix() {
	defer func() {
		r := recover()
		s.Require().NotNil(r)
		s.Require().Equal(errors.New("not allowed empty prefix"), r)
	}()
	s.factory.NewCache([]Setting{{Prefix: ""}})
}

func (s *factorySuite) TestCreateWithDuplicatedPrefix() {
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

func (s *factorySuite) TestCreateWithOnlyMarshal() {
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

func (s *factorySuite) TestCreateWithOnlyUnmarshal() {
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
