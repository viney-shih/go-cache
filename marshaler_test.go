package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

var (
	mockTimeNow = time.Date(2022, 11, 23, 0, 0, 0, 0, time.Local)
)

type marshalerSuite struct {
	suite.Suite
}

func (s *marshalerSuite) SetupSuite() {}

func (s *marshalerSuite) TearDownSuite() {}

func (s *marshalerSuite) SetupTest() {}

func (s *marshalerSuite) TearDownTest() {}

func TestMarshalerSuite(t *testing.T) {
	suite.Run(t, new(marshalerSuite))
}

type mockStruct struct {
	ID        int64
	Key       string
	CreatedAt time.Time
	child     *mockStruct
}

func (s *marshalerSuite) TestMarshaler() {
	var bs []byte
	var err error
	marshal := Marshal
	unmarshal := Unmarshal

	// nil
	var null error
	bs, err = marshal(null)
	s.Require().NoError(err)

	var retNull error
	s.Require().NoError(unmarshal(bs, &retNull))
	s.Require().Equal(null, retNull)

	// bytes
	bytes := []byte("strings to bytes")
	bs, err = marshal(bytes)
	s.Require().NoError(err)

	var retBytes []byte
	s.Require().NoError(unmarshal(bs, &retBytes))
	s.Require().Equal(bytes, retBytes)

	// string
	str := "this is a string"
	bs, err = marshal(str)
	s.Require().NoError(err)

	var retStr string
	s.Require().NoError(unmarshal(bs, &retStr))
	s.Require().Equal(str, retStr)

	// pointer
	num := 100
	intPtr := &num
	bs, err = marshal(intPtr)
	s.Require().NoError(err)

	var retIntPtr *int
	s.Require().NoError(unmarshal(bs, &retIntPtr))
	s.Require().Equal(intPtr, retIntPtr)

	// struct
	st := mockStruct{
		ID:        28825252,
		Key:       "I am rich",
		CreatedAt: mockTimeNow,
	}
	bs, err = marshal(st)
	s.Require().NoError(err)

	retSt := mockStruct{}
	s.Require().NoError(unmarshal(bs, &retSt))
	s.Require().Equal(st, retSt)

	// struct without nil pointer
	st2 := mockStruct{
		ID:        28825252,
		Key:       "I am rich",
		CreatedAt: mockTimeNow,
		child: &mockStruct{
			ID: 2266,
		},
	}
	bs, err = marshal(st2)
	s.Require().NoError(err)

	var retSt2 mockStruct
	s.Require().NoError(unmarshal(bs, &retSt2))
	s.Require().Equal(st, retSt2)

	// compress
	st3 := mockStruct{
		ID:        1234567890,
		Key:       `1234567890123456789012345678901234567890123456789012345678901234567890`, // 70 chars
		CreatedAt: mockTimeNow,
	}
	bs, err = marshal(st3)
	s.Require().NoError(err)

	var retSt3 mockStruct
	s.Require().NoError(unmarshal(bs, &retSt3))
	s.Require().Equal(st3, retSt3)
}
