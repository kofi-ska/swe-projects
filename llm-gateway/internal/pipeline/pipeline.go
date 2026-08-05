package pipeline

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

var ErrQueueFull = errors.New("request queue is full")

type Event struct {
	Token  string
	Done   bool
	Tokens int
	Err    error
}
type Request struct {
	Context   context.Context
	MaxTokens int
	Priority  string
	Enqueued  time.Time
	Events    chan Event
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
	wg        sync.WaitGroup
	running   atomic.Bool
}

func New(capacity, maxBatch, workers int, maxWait time.Duration, p Processor, o Observer) *Pipeline {
	if o == nil {
		o = noopObserver{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Pipeline{queue: make(chan *Request, capacity), batches: make(chan []*Request, workers), maxBatch: maxBatch, maxWait: maxWait, workers: workers, processor: p, observer: o, ctx: ctx, cancel: cancel}
}

func (p *Pipeline) Start() {
	if p.running.Swap(true) {
		return
	}
	p.wg.Add(1)
	go p.batchLoop()
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.workerLoop(i)
	}
}

func (p *Pipeline) Enqueue(r *Request) error {
	if !p.running.Load() {
		return ErrQueueFull
	}
	select {
	case p.queue <- r:
		p.observer.QueueDepth(len(p.queue))
		return nil
	default:
		return ErrQueueFull
	}
}

func (p *Pipeline) Stop() {
	if p.running.Swap(false) {
		p.cancel()
		p.wg.Wait()
	}
}
func (p *Pipeline) Running() bool      { return p.running.Load() }
func (p *Pipeline) QueueDepth() int    { return len(p.queue) }
func (p *Pipeline) QueueCapacity() int { return cap(p.queue) }
func (p *Pipeline) Workers() int       { return p.workers }

func (p *Pipeline) batchLoop() {
	defer p.wg.Done()
	defer close(p.batches)
	for {
		select {
		case <-p.ctx.Done():
			return
		case first := <-p.queue:
			started := time.Now()
			batch := []*Request{first}
			timer := time.NewTimer(p.maxWait)
		collect:
			for len(batch) < p.maxBatch {
				select {
				case r := <-p.queue:
					batch = append(batch, r)
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
			for _, r := range batch {
				if r.Context.Err() == nil {
					p.observer.QueueWait(time.Since(r.Enqueued))
					live = append(live, r)
				}
			}
			p.observer.QueueDepth(len(p.queue))
			if len(live) == 0 {
				continue
			}
			p.observer.Batch(len(live), time.Since(started))
			select {
			case p.batches <- live:
			case <-p.ctx.Done():
				return
			}
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
