package pipeline

import (
	"context"
	"sync"
	"testing"
	"time"
)

type recorder struct {
	mu      sync.Mutex
	batches []int
	called  chan struct{}
}

func (r *recorder) Process(_ context.Context, _ int, b []*Request) {
	r.mu.Lock()
	r.batches = append(r.batches, len(b))
	r.mu.Unlock()
	select {
	case r.called <- struct{}{}:
	default:
	}
}
func request(ctx context.Context) *Request {
	return &Request{Context: ctx, Enqueued: time.Now(), Events: make(chan Event, 1), MaxTokens: 1}
}

func TestBatchMaximumSize(t *testing.T) {
	r := &recorder{called: make(chan struct{}, 1)}
	p := New(10, 3, 1, time.Second, r, nil)
	p.Start()
	defer p.Stop()
	for i := 0; i < 3; i++ {
		if err := p.Enqueue(request(context.Background())); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-r.called:
	case <-time.After(time.Second):
		t.Fatal("batch not processed")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if got := r.batches[0]; got != 3 {
		t.Fatalf("batch size=%d, want 3", got)
	}
}

func TestBatchMaximumWait(t *testing.T) {
	r := &recorder{called: make(chan struct{}, 1)}
	p := New(10, 8, 1, 25*time.Millisecond, r, nil)
	p.Start()
	defer p.Stop()
	started := time.Now()
	if err := p.Enqueue(request(context.Background())); err != nil {
		t.Fatal(err)
	}
	select {
	case <-r.called:
		if elapsed := time.Since(started); elapsed < 20*time.Millisecond || elapsed > 250*time.Millisecond {
			t.Fatalf("batch elapsed %s", elapsed)
		}
	case <-time.After(time.Second):
		t.Fatal("batch not processed")
	}
}

func TestCancelledQueuedRequestIsDropped(t *testing.T) {
	r := &recorder{called: make(chan struct{}, 1)}
	p := New(10, 2, 1, 20*time.Millisecond, r, nil)
	p.Start()
	defer p.Stop()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = p.Enqueue(request(ctx))
	_ = p.Enqueue(request(context.Background()))
	select {
	case <-r.called:
	case <-time.After(time.Second):
		t.Fatal("batch not processed")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if got := r.batches[0]; got != 1 {
		t.Fatalf("processed %d requests, want 1", got)
	}
}

func BenchmarkBatcher(b *testing.B) {
	r := &recorder{called: make(chan struct{}, b.N)}
	p := New(b.N+1, 8, 4, time.Microsecond, r, nil)
	p.Start()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.Enqueue(request(context.Background()))
	}
	p.Stop()
}
