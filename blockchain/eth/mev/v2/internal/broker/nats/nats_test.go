package nats

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"mevrelayv2/internal/broker"
)

func TestPublishSubscribeAndPing(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.Write([]byte("INFO {\"server_id\":\"test\"}\r\n"))
		reader := bufio.NewReader(conn)
		subs := map[string]int{}
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch {
			case strings.HasPrefix(line, "CONNECT "):
				continue
			case line == "PING":
				_, _ = conn.Write([]byte("PONG\r\n"))
			case strings.HasPrefix(line, "SUB "):
				parts := strings.Fields(line)
				if len(parts) >= 4 {
					subject := parts[1]
					sid := parts[len(parts)-1]
					if sidn, err := strconv.Atoi(sid); err == nil {
						subs[subject] = sidn
					}
				}
			case strings.HasPrefix(line, "PUB "):
				parts := strings.Fields(line)
				if len(parts) < 3 {
					return
				}
				subject := parts[1]
				size := 0
				if n, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
					size = n
				}
				payload := make([]byte, size+2)
				if _, err := io.ReadFull(reader, payload); err != nil {
					return
				}
				sid := subs[subject]
				msg := fmt.Sprintf("MSG %s %d %d\r\n%s\r\n", subject, sid, size, string(payload[:size]))
				_, _ = conn.Write([]byte(msg))
			}
		}
	}()

	br, err := New("nats://" + ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer br.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := br.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	consumer, err := br.Subscribe(ctx, "topic", "group")
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()

	if err := br.Publish(ctx, broker.Message{Topic: "topic", Payload: []byte("hello")}); err != nil {
		t.Fatal(err)
	}

	del, err := consumer.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(del.Message().Payload) != "hello" {
		t.Fatalf("unexpected payload: %q", del.Message().Payload)
	}
}
