package cache

import "context"

// Pubsub is the interface to deal with the message queue
type Pubsub interface {
	// Pub publishes the message to the message queue with specified topic
	Pub(context context.Context, topic string, message []byte) error
	// Sub subscribes messages from the message queue with specified topics
	Sub(context context.Context, topic ...string) <-chan Message
	// Close closes the subscription only if Sub() is used.
	// In other word, should handle un-normal usage when Sub() didn't happen before.
	Close()
}

// Message is the interface to receive messages from message queue
type Message interface {
	// Topic returns the topic
	Topic() string
	// Content returns the content of the message
	Content() []byte
}
