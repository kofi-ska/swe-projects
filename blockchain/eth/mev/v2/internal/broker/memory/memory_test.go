package memory

import (
	"context"
	"testing"
	"time"

	"mevrelayv2/internal/broker"
)

func TestPublishSubscribe(t *testing.T) {
	br := New(4)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

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
