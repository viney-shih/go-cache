//go:generate go-enum -f=$GOFILE --nocase

package cache

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

var (
	// errSelfEvent indicates event triggered by itself.
	errSelfEvent = errors.New("event triggered by itself")
	// errNoEventType indicates no event types
	errNoEventType = errors.New("no event types")
)

// eventType is an enumeration of events used to communicate with each other via Pubsub.
/*
ENUM(
None // Not registered Event by default.
Evict // Evict presents eviction event.
)
*/
type eventType int32

var regTopicEventMap map[string]eventType

func init() {
	regTopicEventMap = map[string]eventType{}

	for typ := range _eventTypeMap {
		if typ == EventTypeNone {
			continue
		}

		regTopicEventMap[typ.Topic()] = typ
	}
}

// Topic generates the topic for specified event.
func (x eventType) Topic() string {
	return customKey(topicDelim, packageKey, topicKey, x.String())
}

type event struct {
	Type eventType
	Body eventBody
}

type eventBody struct {
	FID  string
	Keys []string
}

type messageBroker struct {
	pubsub Pubsub
	fid    string
	wg     sync.WaitGroup
}

func newMessageBroker(fid string, pb Pubsub) *messageBroker {
	return &messageBroker{
		fid:    fid,
		pubsub: pb,
	}
}

func (mb *messageBroker) registered() bool {
	return mb.pubsub != nil
}

func (mb *messageBroker) close() {
	if !mb.registered() {
		return
	}

	// close s
	mb.pubsub.Close()
	mb.wg.Wait()
}

func (mb *messageBroker) send(ctx context.Context, e event) error {
	if !mb.registered() {
		return nil
	}

	e.Body.FID = mb.fid
	bs, err := json.Marshal(e.Body)
	if err != nil {
		return err
	}

	return mb.pubsub.Pub(ctx, e.Type.Topic(), bs)
}

func (mb *messageBroker) listen(
	ctx context.Context, types []eventType, cb func(context.Context, *event, error),
) error {
	if !mb.registered() {
		return nil
	}

	if len(types) == 0 {
		return errNoEventType
	}

	topics := make([]string, len(types))
	for i := 0; i < len(types); i++ {
		topics[i] = types[i].Topic()
	}

	mb.wg.Add(1)
	go func() {
		defer mb.wg.Done()

		for mess := range mb.pubsub.Sub(ctx, topics...) {
			typ, ok := regTopicEventMap[mess.Topic()]
			if !ok {
				cb(ctx, nil, errors.New("no such topic registered"))
				continue
			}

			e := event{Type: typ}
			if err := json.Unmarshal(mess.Content(), &e.Body); err != nil {
				cb(ctx, nil, err)
				continue
			}

			if e.Body.FID == mb.fid {
				cb(ctx, &e, errSelfEvent)
				continue
			}

			cb(ctx, &e, nil)
		}
	}()

	return nil
}
