package memory

import (
	"context"
	"errors"
	"sync"

	"mevrelayv3/internal/broker"
)

type Broker struct {
	mu     sync.Mutex
	topics map[string][]chan broker.Message
	closed bool
	buffer int
}

type consumer struct {
	broker *Broker
	topic  string
	ch     chan broker.Message
	closed chan struct{}
}

type delivery struct {
	msg broker.Message
}

func New(buffer int) *Broker {
	if buffer <= 0 {
		buffer = 64
	}
	return &Broker{topics: map[string][]chan broker.Message{}, buffer: buffer}
}

func (b *Broker) Publish(ctx context.Context, msg broker.Message) error {
	_ = ctx
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return errors.New("broker closed")
	}
	subs := append([]chan broker.Message(nil), b.topics[msg.Topic]...)
	b.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- msg:
		default:
		}
	}
	return nil
}

func (b *Broker) Subscribe(_ context.Context, topic, _ string) (broker.Consumer, error) {
	ch := make(chan broker.Message, b.buffer)
	if topic == "" {
		topic = "default"
	}
	c := &consumer{broker: b, topic: topic, ch: ch, closed: make(chan struct{})}
	b.mu.Lock()
	b.topics[c.topic] = append(b.topics[c.topic], ch)
	b.mu.Unlock()
	return c, nil
}

func (b *Broker) Ping(context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("broker closed")
	}
	return nil
}

func (b *Broker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	return nil
}

func (c *consumer) Receive(ctx context.Context) (broker.Delivery, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.closed:
		return nil, errors.New("consumer closed")
	case msg := <-c.ch:
		return delivery{msg: msg}, nil
	}
}

func (c *consumer) Close() error {
	close(c.closed)
	if c.broker != nil {
		c.broker.mu.Lock()
		subs := c.broker.topics[c.topic]
		out := subs[:0]
		for _, ch := range subs {
			if ch != c.ch {
				out = append(out, ch)
			}
		}
		c.broker.topics[c.topic] = out
		c.broker.mu.Unlock()
	}
	return nil
}

func (d delivery) Message() broker.Message      { return d.msg }
func (d delivery) Ack(context.Context) error    { return nil }
func (d delivery) Nack(context.Context) error   { return nil }
