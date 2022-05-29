//go:generate go-enum -f=$GOFILE --nocase

package cache

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

var (
	// ErrSelfEvent indicates event triggered by itself.
	ErrSelfEvent = errors.New("event triggered by itself")
)

// EventType is an enumeration of events used to communicate with each other via Pubsub.
/*
ENUM(
None // Not registered Event by default.
Evict // Evict presents eviction event.
)
*/
type EventType int32

var regTopicEventMap map[string]EventType

func init() {
	regTopicEventMap = map[string]EventType{}

	for typ := range _EventTypeMap {
		if typ == EventTypeNone {
			continue
		}

		regTopicEventMap[typ.Topic()] = typ
	}
}

// Topic generates the topic for specified event.
func (x EventType) Topic() string {
	return customKey(topicDelim, packageKey, topicKey, x.String())
}

type event struct {
	Type EventType
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

	bs, err := json.Marshal(e.Body)
	if err != nil {
		return err
	}

	return mb.pubsub.Pub(ctx, e.Type.Topic(), bs)
}

func (mb *messageBroker) listen(
	ctx context.Context, types []EventType, cb func(context.Context, *event, error),
) {
	if !mb.registered() {
		return
	}

	if len(types) == 0 {
		return
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
				cb(ctx, &e, ErrSelfEvent)
				continue
			}

			cb(ctx, &e, nil)
		}
	}()
}
