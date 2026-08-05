package pipeline

import (
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (p *Pipeline) batchLoop() {
	defer p.wg.Done()
	defer close(p.batches)
	for {
		first, open := <-p.queue
		if !open {
			return
		}
		started := time.Now()
		batch, queueClosed := p.collectBatch(first)
		p.dispatch(batch, time.Since(started))
		if queueClosed {
			return
		}
	}
}

func (p *Pipeline) collectBatch(first *Request) ([]*Request, bool) {
	batch := make([]*Request, 0, p.maxBatch)
	batch = append(batch, first)
	timer := time.NewTimer(p.maxWait)
	defer timer.Stop()
	for len(batch) < p.maxBatch {
		select {
		case request, open := <-p.queue:
			if !open {
				return batch, true
			}
			batch = append(batch, request)
		case <-timer.C:
			return batch, false
		case <-p.ctx.Done():
			return nil, true
		}
	}
	return batch, false
}

func (p *Pipeline) dispatch(batch []*Request, batchWait time.Duration) {
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
	if len(live) == 0 {
		return
	}
	p.observer.Batch(len(live), batchWait)
	for _, request := range live {
		_, span := otel.Tracer("llm-gateway/pipeline").Start(request.ctx, "form-batch")
		span.SetAttributes(attribute.Int("batch.size", len(live)), attribute.Int("queue.depth", len(p.queue)))
		span.AddEvent("batch.formed")
		span.End()
	}
	select {
	case p.batches <- live:
	case <-p.ctx.Done():
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
