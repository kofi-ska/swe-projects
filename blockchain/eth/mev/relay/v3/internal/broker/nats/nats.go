package nats

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"mevrelayv3/internal/broker"
)

type Broker struct {
	mu       sync.Mutex
	conn     net.Conn
	reader   *bufio.Reader
	writer   *bufio.Writer
	subs     map[int]*consumer
	nextSID  int
	closed   bool
	pingWait map[string]chan error
}

func New(url string) (*Broker, error) {
	addr, err := parseNATSURL(url)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return nil, err
	}
	b := &Broker{
		conn:     conn,
		reader:   bufio.NewReaderSize(conn, 1<<20),
		writer:   bufio.NewWriterSize(conn, 1<<20),
		subs:     map[int]*consumer{},
		pingWait: map[string]chan error{},
	}
	if err := b.handshake(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	go b.readLoop()
	return b, nil
}

func parseNATSURL(url string) (string, error) {
	if strings.HasPrefix(url, "nats://") {
		return strings.TrimPrefix(url, "nats://"), nil
	}
	if url != "" {
		return url, nil
	}
	return "", errors.New("empty nats url")
}

func (b *Broker) handshake() error {
	if err := b.writeLine(`CONNECT {"verbose":false,"pedantic":false,"lang":"go","version":"v3","echo":false}`); err != nil {
		return err
	}
	if err := b.writeLine("PING"); err != nil {
		return err
	}
	if err := b.writer.Flush(); err != nil {
		return err
	}
	for {
		line, err := b.readLine()
		if err != nil {
			return err
		}
		switch {
		case strings.HasPrefix(line, "INFO "):
			continue
		case line == "PONG":
			return nil
		case strings.HasPrefix(line, "-ERR"):
			return errors.New(line)
		}
	}
}

func (b *Broker) Publish(ctx context.Context, msg broker.Message) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("broker closed")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := b.writeMsg(msg.Topic, msg.Payload); err != nil {
		return err
	}
	return b.writer.Flush()
}

func (b *Broker) Subscribe(ctx context.Context, topic, group string) (broker.Consumer, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, errors.New("broker closed")
	}
	sid := b.nextSID
	b.nextSID++
	c := &consumer{
		id: sid,
		ch: make(chan broker.Message, 1024),
		b:  b,
		t:  topic,
		g:  group,
	}
	b.subs[sid] = c
	cmd := fmt.Sprintf("SUB %s", topic)
	if group != "" {
		cmd = fmt.Sprintf("SUB %s %s", topic, group)
	}
	if err := b.writeLine(cmd + " " + strconv.Itoa(sid)); err != nil {
		delete(b.subs, sid)
		return nil, err
	}
	if err := b.writer.Flush(); err != nil {
		delete(b.subs, sid)
		return nil, err
	}
	return c, nil
}

func (b *Broker) Ping(ctx context.Context) error {
	token := strconv.FormatInt(time.Now().UnixNano(), 10)
	wait := make(chan error, 1)
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return errors.New("broker closed")
	}
	b.pingWait[token] = wait
	if err := b.writeLine("PING"); err != nil {
		delete(b.pingWait, token)
		b.mu.Unlock()
		return err
	}
	if err := b.writer.Flush(); err != nil {
		delete(b.pingWait, token)
		b.mu.Unlock()
		return err
	}
	b.mu.Unlock()

	select {
	case err := <-wait:
		b.mu.Lock()
		delete(b.pingWait, token)
		b.mu.Unlock()
		return err
	case <-ctx.Done():
		b.mu.Lock()
		delete(b.pingWait, token)
		b.mu.Unlock()
		return ctx.Err()
	case <-time.After(2 * time.Second):
		b.mu.Lock()
		delete(b.pingWait, token)
		b.mu.Unlock()
		return errors.New("ping timeout")
	}
}

func (b *Broker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	for _, c := range b.subs {
		close(c.ch)
	}
	b.subs = nil
	if b.conn != nil {
		return b.conn.Close()
	}
	return nil
}

func (b *Broker) writeLine(line string) error {
	_, err := b.writer.WriteString(line + "\r\n")
	return err
}

func (b *Broker) writeMsg(subject string, payload []byte) error {
	if err := b.writeLine(fmt.Sprintf("PUB %s %d", subject, len(payload))); err != nil {
		return err
	}
	if _, err := b.writer.Write(payload); err != nil {
		return err
	}
	return b.writeLine("")
}

func (b *Broker) readLine() (string, error) {
	line, err := b.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func (b *Broker) readLoop() {
	for {
		line, err := b.readLine()
		if err != nil {
			if err != io.EOF {
				_ = b.Close()
			}
			return
		}
		switch {
		case line == "PING":
			b.mu.Lock()
			_ = b.writeLine("PONG")
			_ = b.writer.Flush()
			b.mu.Unlock()
		case line == "PONG":
			b.mu.Lock()
			for token, wait := range b.pingWait {
				select {
				case wait <- nil:
				default:
				}
				delete(b.pingWait, token)
			}
			b.mu.Unlock()
		case strings.HasPrefix(line, "MSG "):
			if err := b.handleMsg(line); err != nil {
				_ = b.Close()
				return
			}
		case strings.HasPrefix(line, "-ERR"):
			_ = b.Close()
			return
		}
	}
}

func (b *Broker) handleMsg(line string) error {
	parts := strings.Fields(line)
	if len(parts) < 4 {
		return errors.New("invalid msg frame")
	}
	sid, err := strconv.Atoi(parts[2])
	if err != nil {
		return err
	}
	sizeStr := parts[len(parts)-1]
	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		return err
	}
	payload := make([]byte, size+2)
	if _, err := io.ReadFull(b.reader, payload); err != nil {
		return err
	}
	payload = payload[:size]

	b.mu.Lock()
	c := b.subs[sid]
	b.mu.Unlock()
	if c == nil {
		return nil
	}
	select {
	case c.ch <- broker.Message{Topic: c.t, Payload: append([]byte(nil), payload...)}:
	default:
	}
	return nil
}

type consumer struct {
	id   int
	ch   chan broker.Message
	b    *Broker
	t    string
	g    string
	once sync.Once
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
	c.once.Do(func() {
		c.b.mu.Lock()
		defer c.b.mu.Unlock()
		delete(c.b.subs, c.id)
		close(c.ch)
	})
	return nil
}

type delivery struct{ msg broker.Message }

func (d *delivery) Message() broker.Message    { return d.msg }
func (d *delivery) Ack(context.Context) error  { return nil }
func (d *delivery) Nack(context.Context) error { return nil }

func (b *Broker) Health(ctx context.Context) error { return b.Ping(ctx) }

var _ broker.Broker = (*Broker)(nil)
