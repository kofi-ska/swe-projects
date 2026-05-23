package memory

import (
	"context"
	"errors"
	"sync"

	"mevrelayv2/internal/broker"
)

// Broker is a bounded in-memory broker for local testing and single-process runs.
type Broker struct {
	mu          sync.RWMutex
	closed      bool
	buffer      int
	subscribers map[string]map[*consumer]struct{}
}

// New creates an in-memory broker with the given subscriber buffer size.
func New(buffer int) *Broker {
	if buffer <= 0 {
		buffer = 64
	}
	return &Broker{
		buffer:      buffer,
		subscribers: make(map[string]map[*consumer]struct{}),
	}
}

// Publish fans out one message to all topic subscribers.
func (b *Broker) Publish(ctx context.Context, msg broker.Message) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return errors.New("broker closed")
	}
	subs := b.subscribers[msg.Topic]
	for c := range subs {
		select {
		case c.ch <- cloneMessage(msg):
		case <-ctx.Done():
			return ctx.Err()
		default:
			return errors.New("broker subscriber buffer full")
		}
	}
	return nil
}

// Subscribe registers a consumer for the topic.
func (b *Broker) Subscribe(ctx context.Context, topic, group string) (broker.Consumer, error) {
	_ = ctx
	_ = group
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, errors.New("broker closed")
	}
	c := &consumer{
		ch: make(chan broker.Message, b.buffer),
		b:  b,
		t:  topic,
	}
	if _, ok := b.subscribers[topic]; !ok {
		b.subscribers[topic] = make(map[*consumer]struct{})
	}
	b.subscribers[topic][c] = struct{}{}
	return c, nil
}

// Ping reports health for the in-memory broker.
func (b *Broker) Ping(context.Context) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return errors.New("broker closed")
	}
	return nil
}

// Close releases all subscribers.
func (b *Broker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	for _, subs := range b.subscribers {
		for c := range subs {
			close(c.ch)
		}
	}
	b.subscribers = nil
	return nil
}

type consumer struct {
	ch     chan broker.Message
	closed bool
	mu     sync.Mutex
	b      *Broker
	t      string
}

func (c *consumer) Receive(ctx context.Context) (broker.Delivery, error) {
	select {
	case msg, ok := <-c.ch:
		if !ok {
			return nil, errors.New("broker consumer closed")
		}
		return &delivery{msg: msg}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *consumer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	c.b.mu.Lock()
	defer c.b.mu.Unlock()
	if subs := c.b.subscribers[c.t]; subs != nil {
		delete(subs, c)
	}
	close(c.ch)
	return nil
}

type delivery struct {
	msg broker.Message
}

func (d *delivery) Message() broker.Message    { return cloneMessage(d.msg) }
func (d *delivery) Ack(context.Context) error  { return nil }
func (d *delivery) Nack(context.Context) error { return nil }

func cloneMessage(msg broker.Message) broker.Message {
	clone := msg
	if len(msg.Headers) > 0 {
		clone.Headers = make(map[string]string, len(msg.Headers))
		for k, v := range msg.Headers {
			clone.Headers[k] = v
		}
	}
	if len(msg.Payload) > 0 {
		clone.Payload = append([]byte(nil), msg.Payload...)
	}
	return clone
}
