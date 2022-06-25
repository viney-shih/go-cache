package cache

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/suite"
)

type keySuite struct {
	suite.Suite
}

func (s *keySuite) SetupSuite() {}

func (s *keySuite) TearDownSuite() {}

func (s *keySuite) SetupTest() {}

func (s *keySuite) TearDownTest() {
	clearRegisteredKey()
}

func TestKeySuite(t *testing.T) {
	suite.Run(t, new(keySuite))
}

func clearRegisteredKey() {
	regPkgKey = packageKey
	regKeyOnce = sync.Once{}
}

func (s *keySuite) TestGetPrefixAndKey() {
	tests := []struct {
		Desc     string
		CacheKey string
		ExpPfx   string
		ExpKey   string
	}{
		{
			Desc:     "invalid cache key without delimiter",
			CacheKey: "12345",
			ExpPfx:   "12345",
			ExpKey:   "",
		},
		{
			Desc:     "invalid cache key with only one delimiter",
			CacheKey: fmt.Sprintf("%s%s%s", "123", cacheDelim, "abc"),
			ExpPfx:   "abc",
			ExpKey:   "",
		},
		{
			Desc:     "normal case",
			CacheKey: getCacheKey("prefix", "key"),
			ExpPfx:   "prefix",
			ExpKey:   "key",
		},
	}

	for _, t := range tests {
		pfx, key := getPrefixAndKey(t.CacheKey)
		s.Require().Equal(t.ExpPfx, pfx, t.Desc)
		s.Require().Equal(t.ExpKey, key, t.Desc)

		s.TearDownTest()
	}
}

func (s *keySuite) TestRegister() {
	s.Require().Equal(packageKey, regPkgKey)

	Register("specified")
	s.Require().Equal("specified", regPkgKey)

	Register("another")
	s.Require().Equal("specified", regPkgKey) // no change

	clearRegisteredKey()
	s.Require().Equal(packageKey, regPkgKey) // set to default

	Register("another")
	s.Require().Equal("another", regPkgKey) // set to another
}

func (s *keySuite) TestRegisterAndGetCacheKey() {
	var cKey, pfx, key string

	s.Require().Equal(fmt.Sprintf("%s:pfx:key", packageKey), getCacheKey("pfx", "key"))

	Register("my")
	cKey = getCacheKey("pfx", "key")
	s.Require().Equal("my:pfx:key", cKey)
	pfx, key = getPrefixAndKey(cKey)
	s.Require().Equal(pfx, "pfx")
	s.Require().Equal(key, "key")

	clearRegisteredKey()
	s.Require().Equal(fmt.Sprintf("%s:pfx:key", packageKey), getCacheKey("pfx", "key")) // set to default

	Register("") // empty package key
	cKey = getCacheKey("pfx", "key")
	s.Require().Equal("pfx:key", cKey)
	pfx, key = getPrefixAndKey(cKey)
	s.Require().Equal(pfx, "pfx")
	s.Require().Equal(key, "key")
}
