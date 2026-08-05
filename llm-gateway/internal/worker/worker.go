package worker

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/kofi-ska/llm-gateway/internal/pipeline"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Worker struct {
	BaseDelay, TokenDelay time.Duration
	FailureRate           float64
}

var vocabulary = []string{" Distributed", " systems", " coordinate", " work", " across", " independent", " components", " while", " preserving", " reliability", ",", " visibility", ", and", " controlled", " failure", "."}

func (w Worker) Process(parent context.Context, id int, batch []*pipeline.Request) {
	type requestSpan struct {
		request *pipeline.Request
		span    trace.Span
	}
	spans := make([]requestSpan, 0, len(batch))
	for _, r := range batch {
		_, span := otel.Tracer("llm-gateway/worker").Start(r.Context, "fake-model-inference")
		span.SetAttributes(attribute.Int("worker.id", id), attribute.Int("batch.size", len(batch)))
		spans = append(spans, requestSpan{request: r, span: span})
	}
	defer func() {
		for _, item := range spans {
			item.span.End()
		}
	}()
	if !sleep(parent, w.BaseDelay) {
		return
	}
	if rand.Float64() < w.FailureRate {
		for _, item := range spans {
			item.span.SetStatus(codes.Error, "simulated inference failure")
		}
		for _, r := range batch {
			send(r, pipeline.Event{Err: fmt.Errorf("worker %d simulated failure", id)})
		}
		return
	}
	max := 0
	for _, r := range batch {
		if r.MaxTokens > max {
			max = r.MaxTokens
		}
	}
	for i := 0; i < max; i++ {
		if !sleep(parent, w.TokenDelay) {
			return
		}
		active := false
		for _, r := range batch {
			if r.Context.Err() != nil || i >= r.MaxTokens {
				continue
			}
			active = true
			if !send(r, pipeline.Event{Token: vocabulary[i%len(vocabulary)]}) {
				continue
			}
			if i+1 == r.MaxTokens {
				send(r, pipeline.Event{Done: true, Tokens: r.MaxTokens})
			}
		}
		if !active {
			return
		}
	}
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
func send(r *pipeline.Request, e pipeline.Event) bool {
	select {
	case <-r.Context.Done():
		return false
	case r.Events <- e:
		return true
	}
}
