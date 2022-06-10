package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

const (
	mockEmptyPfx = "empty-pfx"
	mockEmptyKey = "empty-key"
)

var (
	mockEmptyCTX = context.Background()
)

type emptySuite struct {
	suite.Suite
}

func (s *emptySuite) SetupSuite() {
}

func (s *emptySuite) TearDownSuite() {}

func (s *emptySuite) SetupTest() {}

func (s *emptySuite) TearDownTest() {
	// prevent registering twice
	ClearPrefix()
}

func TestEmptySuite(t *testing.T) {
	suite.Run(t, new(emptySuite))
}

func (s *emptySuite) TestEmptyAdapter() {
	f := NewFactory(NewEmpty(), NewEmpty())
	c := f.NewCache([]Setting{
		{
			Prefix: mockEmptyPfx,
			CacheAttributes: map[Type]Attribute{
				SharedCacheType: {time.Hour},
				LocalCacheType:  {10 * time.Second},
			},
		},
	})

	var intf interface{}
	s.Require().Equal(ErrCacheMiss, c.Get(mockEmptyCTX, mockEmptyPfx, mockEmptyKey, &intf))
	s.Require().NoError(c.Set(mockEmptyCTX, mockEmptyPfx, mockEmptyKey, 123))
	s.Require().NoError(c.Del(mockEmptyCTX, mockEmptyPfx, mockEmptyKey))
}
