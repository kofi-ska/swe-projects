package pipeline

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrQueueFull = errors.New("request queue is full")
	ErrClosed    = errors.New("pipeline is closed")
)

// Pipeline owns queue admission, batching, worker lifetime, and shutdown.
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

// Shutdown drains accepted work. At the caller's deadline it cancels all
// owned work and still waits for every owned goroutine to exit.
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
