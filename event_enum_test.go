package cache

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type enumSuite struct {
	suite.Suite
}

func (s *enumSuite) SetupSuite() {}

func (s *enumSuite) TearDownSuite() {}

func (s *enumSuite) SetupTest() {}

func (s *enumSuite) TearDownTest() {}

func TestEnumSuite(t *testing.T) {
	suite.Run(t, new(enumSuite))
}

func (s *enumSuite) TestString() {
	s.Require().Equal("Evict", EventTypeEvict.String())

	notExisted := EventType(1000)
	s.Require().Equal("EventType(1000)", notExisted.String())
}

func (s *enumSuite) TestParseEventType() {
	var typ EventType
	var err error

	// normal case
	typ, err = ParseEventType("Evict")
	s.Require().NoError(err)
	s.Require().Equal(EventTypeEvict, typ)

	// lower case
	typ, err = ParseEventType("evict")
	s.Require().NoError(err)
	s.Require().Equal(EventTypeEvict, typ)

	// upper case
	typ, err = ParseEventType("NONE")
	s.Require().NoError(err)
	s.Require().Equal(EventTypeNone, typ)

	// err
	_, err = ParseEventType("not-existed")
	s.Require().Equal(fmt.Errorf("not-existed is not a valid EventType"), err)
}
