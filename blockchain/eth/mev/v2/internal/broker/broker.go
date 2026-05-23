package broker

import (
	"context"
	"time"
)

// Message is the broker-agnostic unit of publication.
type Message struct {
	Topic     string
	Key       string
	Sequence  uint64
	Timestamp time.Time
	Headers   map[string]string
	Payload   []byte
}

// Delivery represents one message handed to a consumer.
type Delivery interface {
	Message() Message
	Ack(context.Context) error
	Nack(context.Context) error
}

// Consumer receives broker deliveries.
type Consumer interface {
	Receive(context.Context) (Delivery, error)
	Close() error
}

// Broker publishes and subscribes messages without exposing transport specifics.
type Broker interface {
	Publish(context.Context, Message) error
	Subscribe(context.Context, string, string) (Consumer, error)
	Ping(context.Context) error
	Close() error
}
