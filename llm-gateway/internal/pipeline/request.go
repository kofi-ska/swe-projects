package pipeline

import (
	"context"
	"time"
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
