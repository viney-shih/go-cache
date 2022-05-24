package cache

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/suite"
)

const (
	mockRdsString = "mock-string"
)

var (
	mockRdsCTX   = context.Background()
	mockRdsBytes = []byte(mockRdsString)
)

type redisSuite struct {
	suite.Suite

	ring *redis.Ring
	rds  *rds
}

func (s *redisSuite) SetupSuite() {
	s.ring = redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{
			"server1": ":6379",
		},
	})
}

func (s *redisSuite) TearDownSuite() {}

func (s *redisSuite) SetupTest() {
	s.rds = NewRedis(s.ring).(*rds)
}

func (s *redisSuite) TearDownTest() {
	_ = s.ring.ForEachShard(mockRdsCTX, func(ctx context.Context, client *redis.Client) error {
		return client.FlushDB(ctx).Err()
	})
}

func TestRedisSuite(t *testing.T) {
	suite.Run(t, new(redisSuite))
}

func (s *redisSuite) TestMGet() {
	tests := []struct {
		Desc      string
		SetupTest func(string)
		Keys      []string
		ExpError  error
		ExpResult []Value
	}{
		{
			Desc:      "not existed",
			Keys:      []string{"not-existed"},
			ExpError:  nil,
			ExpResult: []Value{{Valid: false, Bytes: nil}},
		},
		{
			// diff from tinyLFU, because any values will be converted into string format in redis
			Desc: "invalid format",
			SetupTest: func(desc string) {
				s.Require().NoError(s.ring.Set(mockRdsCTX, "invalid", 80, time.Hour).Err(), desc)
			},
			Keys:      []string{"invalid"},
			ExpError:  nil,
			ExpResult: []Value{{Valid: true, Bytes: []byte(strconv.Itoa(80))}},
		},
		{
			Desc: "empty bytes",
			SetupTest: func(desc string) {
				s.Require().NoError(s.ring.Set(mockRdsCTX, "empty-bytes", []byte{}, time.Hour).Err(), desc)
			},
			Keys:      []string{"empty-bytes"},
			ExpError:  nil,
			ExpResult: []Value{{Valid: true, Bytes: []byte{}}},
		},
		{
			Desc: "normal get",
			SetupTest: func(desc string) {
				s.Require().NoError(s.ring.Set(mockRdsCTX, "normal-get", mockRdsBytes, time.Hour).Err(), desc)
			},
			Keys:      []string{"normal-get"},
			ExpError:  nil,
			ExpResult: []Value{{Valid: true, Bytes: mockRdsBytes}},
		},
	}

	for _, t := range tests {
		if t.SetupTest != nil {
			t.SetupTest(t.Desc)
		}

		values, err := s.rds.MGet(mockRdsCTX, t.Keys)
		s.Require().Equal(t.ExpError, err, t.Desc)
		if err == nil {
			s.Require().Equal(t.ExpResult, values, t.Desc)
		}

		s.TearDownTest()
	}
}

func (s *redisSuite) TestMSet() {
	tests := []struct {
		Desc      string
		KeyVals   map[string][]byte
		TTL       time.Duration
		ExpError  error
		CheckFunc func(string)
	}{
		{
			Desc: "set empty",
			KeyVals: map[string][]byte{
				"set-empty": nil,
			},
			TTL:      time.Hour,
			ExpError: nil,
			CheckFunc: func(desc string) {
				b, err := s.ring.Get(mockRdsCTX, "set-empty").Bytes()
				s.Require().NoError(err, desc)
				s.Require().Equal([]byte{}, b, desc)
			},
		},
		{
			Desc:     "set nothing",
			KeyVals:  map[string][]byte{},
			TTL:      time.Hour,
			ExpError: nil,
			CheckFunc: func(desc string) {
				b, err := s.ring.Get(mockRdsCTX, "set-nothing").Bytes()
				var nilBytes []byte
				s.Require().Equal(redis.Nil, err, desc)
				s.Require().Equal(nilBytes, b, desc)
			},
		},
		{
			Desc: "normal set",
			KeyVals: map[string][]byte{
				"normal-set": mockLfuBytes,
			},
			TTL:      time.Hour,
			ExpError: nil,
			CheckFunc: func(desc string) {
				b, err := s.ring.Get(mockRdsCTX, "normal-set").Bytes()
				s.Require().NoError(err, desc)
				s.Require().Equal(mockLfuBytes, b, desc)
			},
		},
		{
			Desc: "normal set but expired",
			KeyVals: map[string][]byte{
				"normal-set-expired": mockLfuBytes,
			},
			TTL:      50 * time.Millisecond,
			ExpError: nil,
			CheckFunc: func(desc string) {
				// wait until it expired
				time.Sleep(time.Millisecond * 300)

				b, err := s.ring.Get(mockRdsCTX, "normal-set-expired").Bytes()
				var nilBytes []byte
				s.Require().Equal(redis.Nil, err, desc)
				s.Require().Equal(nilBytes, b, desc)
			},
		},
	}

	for _, t := range tests {
		err := s.rds.MSet(mockLfuCTX, t.KeyVals, t.TTL)
		s.Require().Equal(t.ExpError, err, t.Desc)

		if t.CheckFunc != nil {
			t.CheckFunc(t.Desc)
		}

		s.TearDownTest()
	}
}

func (s *redisSuite) TestDel() {
	tests := []struct {
		Desc      string
		SetupTest func(string)
		Keys      []string
		ExpError  error
		CheckFunc func(string)
	}{
		{
			Desc:     "del not existed",
			Keys:     []string{"del-not-existed"},
			ExpError: nil,
		},
		{
			Desc: "normal del",
			SetupTest: func(desc string) {
				s.Require().NoError(s.ring.Set(mockRdsCTX, "normal-del", mockLfuBytes, time.Hour).Err(), desc)

				// make sure it's in cache
				b, err := s.ring.Get(mockRdsCTX, "normal-del").Bytes()
				s.Require().NoError(err, desc)
				s.Require().Equal(mockLfuBytes, b, desc)
			},
			Keys:     []string{"normal-del"},
			ExpError: nil,
			CheckFunc: func(desc string) {
				b, err := s.ring.Get(mockRdsCTX, "normal-del").Bytes()
				var nilBytes []byte
				s.Require().Equal(redis.Nil, err, desc)
				s.Require().Equal(nilBytes, b, desc)
			},
		},
	}

	for _, t := range tests {
		if t.SetupTest != nil {
			t.SetupTest(t.Desc)
		}

		err := s.rds.Del(mockLfuCTX, t.Keys...)
		s.Require().Equal(t.ExpError, err, t.Desc)

		if t.CheckFunc != nil {
			t.CheckFunc(t.Desc)
		}

		s.TearDownTest()
	}
}
