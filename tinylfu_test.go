package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/vmihailenco/go-tinylfu"
)

const (
	mockLfuString = "mock-string"
)

var (
	mockLfuCTX   = context.Background()
	mockLfuBytes = []byte(mockLfuString)
)

type tinyLFUSuite struct {
	suite.Suite

	lfu *tinyLFU
}

func (s *tinyLFUSuite) SetupSuite() {}

func (s *tinyLFUSuite) TearDownSuite() {}

func (s *tinyLFUSuite) SetupTest() {
	s.lfu = NewTinyLFU(10000).(*tinyLFU)
}

func (s *tinyLFUSuite) TearDownTest() {}

func TestTinyLFUSuite(t *testing.T) {
	suite.Run(t, new(tinyLFUSuite))
}

func (s *tinyLFUSuite) TestMGet() {
	tests := []struct {
		Desc      string
		SetupTest func()
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
			Desc: "invalid format",
			SetupTest: func() {
				s.lfu.lfu.Set(&tinylfu.Item{
					Key:      "invalid",
					Value:    80,
					ExpireAt: time.Now().Add(time.Hour),
				})
			},
			Keys:      []string{"invalid"},
			ExpError:  nil,
			ExpResult: []Value{{Valid: false, Bytes: nil}},
		},
		{
			Desc: "empty bytes",
			SetupTest: func() {
				s.lfu.lfu.Set(&tinylfu.Item{
					Key:      "empty-bytes",
					Value:    []byte{},
					ExpireAt: time.Now().Add(time.Hour),
				})
			},
			Keys:      []string{"empty-bytes"},
			ExpError:  nil,
			ExpResult: []Value{{Valid: true, Bytes: []byte{}}},
		},
		{
			Desc: "normal get",
			SetupTest: func() {
				s.lfu.lfu.Set(&tinylfu.Item{
					Key:      "normal-get",
					Value:    mockLfuBytes,
					ExpireAt: time.Now().Add(time.Hour),
				})
			},
			Keys:      []string{"normal-get"},
			ExpError:  nil,
			ExpResult: []Value{{Valid: true, Bytes: mockLfuBytes}},
		},
	}

	for _, t := range tests {
		if t.SetupTest != nil {
			t.SetupTest()
		}

		values, err := s.lfu.MGet(mockLfuCTX, t.Keys)
		s.Require().Equal(t.ExpError, err, t.Desc)
		if err == nil {
			s.Require().Equal(t.ExpResult, values, t.Desc)
		}

		s.TearDownTest()
	}
}

func (s *tinyLFUSuite) TestMSet() {
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
				"set-empty": {},
			},
			TTL:      time.Hour,
			ExpError: nil,
			CheckFunc: func(desc string) {
				b, exist := s.lfu.lfu.Get("set-empty")
				s.Require().True(exist, desc)
				s.Require().Equal([]byte{}, b, desc)
			},
		},
		{
			Desc:     "set nothing",
			KeyVals:  map[string][]byte{},
			TTL:      time.Hour,
			ExpError: nil,
			CheckFunc: func(desc string) {
				b, exist := s.lfu.lfu.Get("set-nothing")
				s.Require().False(exist, desc)
				s.Require().Equal(nil, b, desc)
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
				b, exist := s.lfu.lfu.Get("normal-set")
				s.Require().True(exist, desc)
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

				b, exist := s.lfu.lfu.Get("normal-set-expired")
				s.Require().False(exist, desc)
				s.Require().Equal(nil, b, desc)
			},
		},
	}

	for _, t := range tests {
		err := s.lfu.MSet(mockLfuCTX, t.KeyVals, t.TTL)
		s.Require().Equal(t.ExpError, err, t.Desc)

		if t.CheckFunc != nil {
			t.CheckFunc(t.Desc)
		}

		s.TearDownTest()
	}
}

func (s *tinyLFUSuite) TestDel() {
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
				s.lfu.lfu.Set(&tinylfu.Item{
					Key:      "normal-del",
					Value:    mockLfuBytes,
					ExpireAt: time.Now().Add(time.Hour),
				})

				// make sure it's in cache
				b, exist := s.lfu.lfu.Get("normal-del")
				s.Require().True(exist, desc)
				s.Require().Equal(mockLfuBytes, b, desc)
			},
			Keys:     []string{"normal-del"},
			ExpError: nil,
			CheckFunc: func(desc string) {
				b, exist := s.lfu.lfu.Get("normal-del")
				s.Require().False(exist, desc)
				s.Require().Equal(nil, b, desc)
			},
		},
	}

	for _, t := range tests {
		if t.SetupTest != nil {
			t.SetupTest(t.Desc)
		}

		err := s.lfu.Del(mockLfuCTX, t.Keys...)
		s.Require().Equal(t.ExpError, err, t.Desc)

		if t.CheckFunc != nil {
			t.CheckFunc(t.Desc)
		}

		s.TearDownTest()
	}
}
