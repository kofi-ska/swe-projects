package worker

import (
	"context"
	"testing"
	"time"

	"github.com/kofi-ska/llm-gateway/internal/pipeline"
)

func TestWorkerGeneratesRequestedTokens(t *testing.T) {
	r := pipeline.NewRequest(context.Background(), 3, "normal")
	done := make(chan struct{})
	go func() { Worker{}.Process(context.Background(), 0, []*pipeline.Request{r}); close(done) }()
	tokens := 0
	for _, event := range rangeEvents(t, r.Events(), 4) {
		if event.Token != "" {
			tokens++
		}
		if event.Done && event.Tokens != 3 {
			t.Fatalf("done tokens=%d", event.Tokens)
		}
	}
	<-done
	if tokens != 3 {
		t.Fatalf("tokens=%d", tokens)
	}
}

func TestWorkerHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := pipeline.NewRequest(ctx, 3, "normal")
	Worker{BaseDelay: time.Millisecond}.Process(context.Background(), 0, []*pipeline.Request{r})
	select {
	case event := <-r.Events():
		t.Fatalf("unexpected event: %+v", event)
	default:
	}
}

func TestSlowConsumerDoesNotBlockBatchSibling(t *testing.T) {
	slowCtx, cancelSlow := context.WithCancel(context.Background())
	slow := pipeline.NewRequest(slowCtx, 10, "normal")
	fast := pipeline.NewRequest(context.Background(), 3, "normal")
	done := make(chan struct{})
	go func() { Worker{}.Process(context.Background(), 0, []*pipeline.Request{slow, fast}); close(done) }()
	events := rangeEvents(t, fast.Events(), 4)
	if !events[len(events)-1].Done {
		t.Fatal("fast sibling did not complete")
	}
	cancelSlow()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker remained blocked after cancellation")
	}
}

func rangeEvents(t *testing.T, events <-chan pipeline.Event, count int) []pipeline.Event {
	t.Helper()
	result := make([]pipeline.Event, 0, count)
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()
	for len(result) < count {
		select {
		case event := <-events:
			result = append(result, event)
		case <-timeout.C:
			t.Fatalf("received %d/%d events", len(result), count)
		}
	}
	return result
}

func BenchmarkTokenGeneration(b *testing.B) {
	w := Worker{}
	for range b.N {
		r := pipeline.NewRequest(context.Background(), 16, "normal")
		done := make(chan struct{})
		go func() {
			for range 17 {
				<-r.Events()
			}
			close(done)
		}()
		w.Process(context.Background(), 0, []*pipeline.Request{r})
		<-done
	}
}

func BenchmarkTokenAggregation(b *testing.B) {
	tokens := make([]string, 500)
	for i := range tokens {
		tokens[i] = vocabulary[i%len(vocabulary)]
	}
	b.Run("repeated-concatenation", func(b *testing.B) {
		for range b.N {
			result := ""
			for _, token := range tokens {
				result += token
			}
			_ = result
		}
	})
	b.Run("preallocated-builder", func(b *testing.B) {
		for range b.N {
			_ = joinTokens(tokens)
		}
	})
}
