package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/vmihailenco/msgpack/v5"
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

func (s *marshalerSuite) TestMsgpack() {
	var bs []byte
	var err error
	marshal := msgpack.Marshal
	unmarshal := msgpack.Unmarshal

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
}

func (s *marshalerSuite) TestMarshaler() {
	var bs []byte
	var err error
	marshal := Marshal
	unmarshal := Unmarshal

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
}
