package pipeline

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type recorder struct {
	mu      sync.Mutex
	batches []int
	called  chan struct{}
	block   <-chan struct{}
}

func (r *recorder) Process(ctx context.Context, _ int, batch []*Request) {
	r.mu.Lock()
	r.batches = append(r.batches, len(batch))
	r.mu.Unlock()
	select {
	case r.called <- struct{}{}:
	default:
	}
	if r.block != nil {
		select {
		case <-r.block:
		case <-ctx.Done():
		}
	}
}

func request(ctx context.Context) *Request { return NewRequest(ctx, 1, "normal") }
func shutdown(t testing.TB, p *Pipeline) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestBatchMaximumSize(t *testing.T) {
	r := &recorder{called: make(chan struct{}, 1)}
	p := New(10, 3, 1, time.Second, r, nil)
	defer shutdown(t, p)
	for range 3 {
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
	defer shutdown(t, p)
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

func TestQueueCapacityAndClosedAdmission(t *testing.T) {
	release := make(chan struct{})
	r := &recorder{called: make(chan struct{}, 1), block: release}
	p := New(1, 1, 1, time.Millisecond, r, nil)
	if err := p.Enqueue(request(context.Background())); err != nil {
		t.Fatal(err)
	}
	<-r.called
	if err := p.Enqueue(request(context.Background())); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for p.QueueDepth() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if err := p.Enqueue(request(context.Background())); err != nil {
		t.Fatal(err)
	}
	if err := p.Enqueue(request(context.Background())); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("error=%v, want ErrQueueFull", err)
	}
	close(release)
	shutdown(t, p)
	if err := p.Enqueue(request(context.Background())); !errors.Is(err, ErrClosed) {
		t.Fatalf("error=%v, want ErrClosed", err)
	}
}

func TestShutdownDrainsAcceptedWork(t *testing.T) {
	release := make(chan struct{})
	r := &recorder{called: make(chan struct{}, 2), block: release}
	p := New(2, 1, 1, time.Millisecond, r, nil)
	_ = p.Enqueue(request(context.Background()))
	<-r.called
	_ = p.Enqueue(request(context.Background()))
	done := make(chan error, 1)
	go func() { done <- p.Shutdown(context.Background()) }()
	select {
	case <-done:
		t.Fatal("shutdown returned before accepted work drained")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not finish")
	}
}

func TestShutdownDeadlineCancelsOwnedWorkers(t *testing.T) {
	release := make(chan struct{})
	r := &recorder{called: make(chan struct{}, 1), block: release}
	p := New(1, 1, 1, time.Millisecond, r, nil)
	if err := p.Enqueue(request(context.Background())); err != nil {
		t.Fatal(err)
	}
	<-r.called
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := p.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v, want deadline exceeded", err)
	}
}

func TestCancelledQueuedRequestIsDropped(t *testing.T) {
	r := &recorder{called: make(chan struct{}, 1)}
	p := New(10, 2, 1, 20*time.Millisecond, r, nil)
	defer shutdown(t, p)
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
	b.ResetTimer()
	for range b.N {
		_ = p.Enqueue(request(context.Background()))
	}
	shutdown(b, p)
}
