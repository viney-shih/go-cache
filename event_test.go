package cache

import (
	"context"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/suite"
)

const (
	mockEventPfx  = "event-pfx"
	mockEventKey  = "event-key"
	mockEventUUID = "event-from-others"
)

var (
	mockEventCTX = context.Background()
)

type eventSuite struct {
	suite.Suite

	factory *factory
	rds     *rds
	lfu     *tinyLFU
	ring    *redis.Ring
	mb      *messageBroker
}

func (s *eventSuite) SetupSuite() {
	s.ring = redis.NewRing(&redis.RingOptions{
		Addrs: map[string]string{
			"server1": ":6379",
		},
	})
}

func (s *eventSuite) TearDownSuite() {}

func (s *eventSuite) SetupTest() {
	s.rds = NewRedis(s.ring).(*rds)
	s.lfu = NewTinyLFU(10000).(*tinyLFU)
	s.mb = newMessageBroker(mockEventUUID, s.rds)
	s.factory = NewFactory(s.rds, s.lfu, WithPubSub(s.rds)).(*factory)
}

func (s *eventSuite) TearDownTest() {
	// prevent registering twice
	ClearPrefix()
	// flush redis
	_ = s.ring.ForEachShard(mockCacheCTX, func(ctx context.Context, client *redis.Client) error {
		return client.FlushDB(ctx).Err()
	})

	s.mb.close() // this makes sure only one (mb or factory) can trigger Close() once without panic
	s.factory.Close()
}

func TestEventSuite(t *testing.T) {
	suite.Run(t, new(eventSuite))
}

func (s *eventSuite) TestSubscribedEventsHandlerWithSet() {
	c := s.factory.NewCache([]Setting{
		{
			Prefix: mockEventPfx,
			CacheAttributes: map[Type]Attribute{
				SharedCacheType: {time.Hour},
				LocalCacheType:  {10 * time.Second},
			},
		},
	})

	// Set() will trigger eviction in other machines
	s.Require().NoError(c.Set(mockEventCTX, mockEventPfx, mockEventKey, 100))
	time.Sleep(time.Millisecond * 100)
	val, err := s.lfu.MGet(mockEventCTX, []string{getCacheKey(mockEventPfx, mockEventKey)})
	s.Require().NoError(err)
	s.Require().Equal([]Value{{Valid: true, Bytes: []byte("100")}}, val) // make sure the local value existed without impacted

	// trigger invalid event type, ignore it directly
	// TODO: handling error messages forwarding in the future
	s.Require().NoError(s.mb.send(mockEventCTX, event{Type: EventTypeNone}))
	time.Sleep(time.Millisecond * 100)
	val, err = s.lfu.MGet(mockEventCTX, []string{getCacheKey(mockEventPfx, mockEventKey)})
	s.Require().NoError(err)
	s.Require().Equal([]Value{{Valid: true, Bytes: []byte("100")}}, val)

	// trigger evict event without keys, nothing happened
	s.Require().NoError(s.mb.send(mockEventCTX, event{
		Type: EventTypeEvict,
		Body: eventBody{Keys: []string{}},
	}))
	time.Sleep(time.Millisecond * 100)
	val, err = s.lfu.MGet(mockEventCTX, []string{getCacheKey(mockEventPfx, mockEventKey)})
	s.Require().NoError(err)
	s.Require().Equal([]Value{{Valid: true, Bytes: []byte("100")}}, val)

	// simulate eviction from other machines
	s.Require().NoError(s.mb.send(mockEventCTX, event{
		Type: EventTypeEvict,
		Body: eventBody{Keys: []string{getCacheKey(mockEventPfx, mockEventKey)}},
	}))
	time.Sleep(time.Millisecond * 100)
	val, err = s.lfu.MGet(mockEventCTX, []string{getCacheKey(mockEventPfx, mockEventKey)})
	s.Require().NoError(err)
	s.Require().Equal([]Value{{}}, val) // local value evicted
}

func (s *eventSuite) TestSubscribedEventsHandlerWithDel() {
	c := s.factory.NewCache([]Setting{
		{
			Prefix: mockEventPfx,
			CacheAttributes: map[Type]Attribute{
				SharedCacheType: {time.Hour},
				LocalCacheType:  {10 * time.Second},
			},
		},
	})

	// Set() will trigger eviction in other machines
	s.Require().NoError(c.Set(mockEventCTX, mockEventPfx, mockEventKey, 100))
	time.Sleep(time.Millisecond * 100)
	val, err := s.lfu.MGet(mockEventCTX, []string{getCacheKey(mockEventPfx, mockEventKey)})
	s.Require().NoError(err)
	s.Require().Equal([]Value{{Valid: true, Bytes: []byte("100")}}, val) // make sure the local value existed without impacted

	// Del is the same behavior as Set(), but the value is killed by itself.
	s.Require().NoError(c.Del(mockEventCTX, mockEventPfx, mockEventKey))
	time.Sleep(time.Millisecond * 100)
	val, err = s.lfu.MGet(mockEventCTX, []string{getCacheKey(mockEventPfx, mockEventKey)})
	s.Require().NoError(err)
	s.Require().Equal([]Value{{}}, val)
}

func (s *eventSuite) TestUnnormalEvent() {
	c := s.factory.NewCache([]Setting{
		{
			Prefix: mockEventPfx,
			CacheAttributes: map[Type]Attribute{
				SharedCacheType: {time.Hour},
				LocalCacheType:  {10 * time.Second},
			},
		},
	})

	// Set() will trigger eviction in other machines
	s.Require().NoError(c.Set(mockEventCTX, mockEventPfx, mockEventKey, 100))
	time.Sleep(time.Millisecond * 100)
	val, err := s.lfu.MGet(mockEventCTX, []string{getCacheKey(mockEventPfx, mockEventKey)})
	s.Require().NoError(err)
	s.Require().Equal([]Value{{Valid: true, Bytes: []byte("100")}}, val) // make sure the local value existed without impacted

	// nothing happened due to no handling on such event
	s.Require().NoError(s.rds.Pub(mockEventCTX, "not-existed", nil))

	// TODO: handle this error in the future
	// invalid json format.
	s.Require().NoError(s.rds.Pub(mockEventCTX, EventTypeEvict.Topic(), []byte("")))
}

func (s *eventSuite) TestListenNoEvents() {
	mb := newMessageBroker(mockEventUUID, s.rds)
	s.Require().Equal(errNoEventType, mb.listen(mockEventCTX, []eventType{}, func(ctx context.Context, e *event, err error) {}))
}
