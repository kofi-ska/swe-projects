package worker

import (
	"context"
	"github.com/kofi-ska/llm-gateway/internal/pipeline"
	"testing"
	"time"
)

func TestWorkerGeneratesRequestedTokens(t *testing.T) {
	r := &pipeline.Request{Context: context.Background(), MaxTokens: 3, Events: make(chan pipeline.Event, 4)}
	Worker{}.Process(context.Background(), 0, []*pipeline.Request{r})
	tokens := 0
	for i := 0; i < 4; i++ {
		e := <-r.Events
		if e.Token != "" {
			tokens++
		}
		if e.Done && e.Tokens != 3 {
			t.Fatalf("done tokens=%d", e.Tokens)
		}
	}
	if tokens != 3 {
		t.Fatalf("tokens=%d", tokens)
	}
}

func TestWorkerHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := &pipeline.Request{Context: ctx, MaxTokens: 3, Events: make(chan pipeline.Event, 4)}
	Worker{BaseDelay: time.Millisecond}.Process(context.Background(), 0, []*pipeline.Request{r})
	if len(r.Events) != 0 {
		t.Fatal("generated tokens for cancelled request")
	}
}

func BenchmarkTokenGeneration(b *testing.B) {
	w := Worker{}
	for i := 0; i < b.N; i++ {
		r := &pipeline.Request{Context: context.Background(), MaxTokens: 16, Events: make(chan pipeline.Event, 17)}
		w.Process(context.Background(), 0, []*pipeline.Request{r})
	}
}
