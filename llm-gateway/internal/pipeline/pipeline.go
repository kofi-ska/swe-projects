package pipeline

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrQueueFull = errors.New("request queue is full")
	ErrClosed    = errors.New("pipeline is closed")
)

type Event struct {
	Token  string
	Done   bool
	Tokens int
	Err    error
}

// Request is a unit of admitted work. Its event stream has deliberately small
// capacity so memory use is bounded and backpressure propagates to the worker.
type Request struct {
	ctx       context.Context
	maxTokens int
	priority  string
	enqueued  time.Time
	events    chan Event
}

func NewRequest(ctx context.Context, maxTokens int, priority string) *Request {
	return &Request{ctx: ctx, maxTokens: maxTokens, priority: priority, enqueued: time.Now(), events: make(chan Event, 1)}
}

func (r *Request) Context() context.Context { return r.ctx }
func (r *Request) MaxTokens() int           { return r.maxTokens }
func (r *Request) Priority() string         { return r.priority }
func (r *Request) Events() <-chan Event     { return r.events }

// Emit blocks on backpressure but always remains cancellable.
func (r *Request) Emit(workerCtx context.Context, event Event) bool {
	select {
	case r.events <- event:
		return true
	case <-r.ctx.Done():
		return false
	case <-workerCtx.Done():
		return false
	}
}

type Observer interface {
	QueueDepth(int)
	QueueWait(time.Duration)
	Batch(int, time.Duration)
	BatchProcessing(time.Duration)
	WorkerDelta(int)
}

type noopObserver struct{}

func (noopObserver) QueueDepth(int)                {}
func (noopObserver) QueueWait(time.Duration)       {}
func (noopObserver) Batch(int, time.Duration)      {}
func (noopObserver) BatchProcessing(time.Duration) {}
func (noopObserver) WorkerDelta(int)               {}

type Processor interface {
	Process(context.Context, int, []*Request)
}

// Pipeline owns queue admission, batching, worker lifetime, and shutdown.
// The admission mutex prevents a send racing with queue closure.
type Pipeline struct {
	queue     chan *Request
	batches   chan []*Request
	maxBatch  int
	maxWait   time.Duration
	workers   int
	processor Processor
	observer  Observer

	ctx       context.Context
	cancel    context.CancelFunc
	admission sync.RWMutex
	closed    bool
	closeOnce sync.Once
	done      chan struct{}
	wg        sync.WaitGroup
}

func New(capacity, maxBatch, workers int, maxWait time.Duration, processor Processor, observer Observer) *Pipeline {
	if observer == nil {
		observer = noopObserver{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pipeline{queue: make(chan *Request, capacity), batches: make(chan []*Request, workers), maxBatch: maxBatch, maxWait: maxWait, workers: workers, processor: processor, observer: observer, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	p.start()
	return p
}

func (p *Pipeline) start() {
	p.wg.Add(1)
	go p.batchLoop()
	for id := range p.workers {
		p.wg.Add(1)
		go p.workerLoop(id)
	}
	go func() { p.wg.Wait(); close(p.done) }()
}

func (p *Pipeline) Enqueue(request *Request) error {
	p.admission.RLock()
	defer p.admission.RUnlock()
	if p.closed {
		return ErrClosed
	}
	select {
	case p.queue <- request:
		p.observer.QueueDepth(len(p.queue))
		return nil
	default:
		return ErrQueueFull
	}
}

// Shutdown first closes admission and drains accepted work. If the caller's
// deadline expires, in-flight work is cancelled and Shutdown waits for every
// owned goroutine to exit before returning.
func (p *Pipeline) Shutdown(ctx context.Context) error {
	p.closeOnce.Do(func() {
		p.admission.Lock()
		p.closed = true
		close(p.queue)
		p.admission.Unlock()
	})
	select {
	case <-p.done:
		p.cancel()
		return nil
	case <-ctx.Done():
		p.cancel()
		<-p.done
		return ctx.Err()
	}
}

func (p *Pipeline) Ready(saturation float64) bool {
	p.admission.RLock()
	defer p.admission.RUnlock()
	return !p.closed && float64(len(p.queue))/float64(cap(p.queue)) < saturation
}
func (p *Pipeline) QueueDepth() int    { return len(p.queue) }
func (p *Pipeline) QueueCapacity() int { return cap(p.queue) }
func (p *Pipeline) Workers() int       { return p.workers }

func (p *Pipeline) batchLoop() {
	defer p.wg.Done()
	defer close(p.batches)
	for {
		first, ok := <-p.queue
		if !ok {
			return
		}
		started := time.Now()
		batch := make([]*Request, 0, p.maxBatch)
		batch = append(batch, first)
		timer := time.NewTimer(p.maxWait)
		queueClosed := false
	collect:
		for len(batch) < p.maxBatch {
			select {
			case request, open := <-p.queue:
				if !open {
					queueClosed = true
					break collect
				}
				batch = append(batch, request)
			case <-timer.C:
				break collect
			case <-p.ctx.Done():
				timer.Stop()
				return
			}
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}

		live := batch[:0]
		for _, request := range batch {
			if request.ctx.Err() != nil {
				continue
			}
			now := time.Now()
			wait := now.Sub(request.enqueued)
			_, span := otel.Tracer("llm-gateway/pipeline").Start(request.ctx, "wait-in-queue", trace.WithTimestamp(request.enqueued))
			span.SetAttributes(attribute.Float64("queue.wait_seconds", wait.Seconds()))
			span.End(trace.WithTimestamp(now))
			p.observer.QueueWait(wait)
			live = append(live, request)
		}
		p.observer.QueueDepth(len(p.queue))
		if len(live) > 0 {
			p.observer.Batch(len(live), time.Since(started))
			for _, request := range live {
				_, span := otel.Tracer("llm-gateway/pipeline").Start(request.ctx, "form-batch")
				span.SetAttributes(attribute.Int("batch.size", len(live)), attribute.Int("queue.depth", len(p.queue)))
				span.AddEvent("batch.formed")
				span.End()
			}
			select {
			case p.batches <- live:
			case <-p.ctx.Done():
				return
			}
		}
		if queueClosed {
			return
		}
	}
}

func (p *Pipeline) workerLoop(id int) {
	defer p.wg.Done()
	for batch := range p.batches {
		started := time.Now()
		p.observer.WorkerDelta(1)
		p.processor.Process(p.ctx, id, batch)
		p.observer.WorkerDelta(-1)
		p.observer.BatchProcessing(time.Since(started))
	}
}
