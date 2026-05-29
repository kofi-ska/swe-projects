package broker

import (
	"context"
	"time"
)

type Message struct {
	Topic     string
	Key       string
	Sequence  uint64
	Timestamp time.Time
	Headers   map[string]string
	Payload   []byte
}

type Delivery interface {
	Message() Message
	Ack(context.Context) error
	Nack(context.Context) error
}

type Consumer interface {
	Receive(context.Context) (Delivery, error)
	Close() error
}

type Broker interface {
	Publish(context.Context, Message) error
	Subscribe(context.Context, string, string) (Consumer, error)
	Ping(context.Context) error
	Close() error
}
