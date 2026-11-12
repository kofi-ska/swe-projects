package worker

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/kofi-ska/llm-gateway/internal/pipeline"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type Worker struct {
	BaseDelay   time.Duration
	TokenDelay  time.Duration
	FailureRate float64
}

var vocabulary = [...]string{" Distributed", " systems", " coordinate", " work", " across", " independent", " components", " while", " preserving", " reliability", ",", " visibility", ", and", " controlled", " failure", "."}

// Process fans a batch out into at most len(batch) goroutines. Since both the
// worker count and batch size are fixed configuration, total fan-out is bounded
// by WORKER_COUNT * MAX_BATCH_SIZE. A slow client cannot stall its siblings.
func (w Worker) Process(ctx context.Context, workerID int, batch []*pipeline.Request) {
	if !sleep(ctx, w.BaseDelay) {
		return
	}
	failed := rand.Float64() < w.FailureRate
	var wg sync.WaitGroup
	wg.Add(len(batch))
	for _, request := range batch {
		go func() {
			defer wg.Done()
			w.processRequest(ctx, workerID, len(batch), request, failed)
		}()
	}
	wg.Wait()
}

func (w Worker) processRequest(ctx context.Context, workerID, batchSize int, request *pipeline.Request, failed bool) {
	_, span := otel.Tracer("llm-gateway/worker").Start(request.Context(), "fake-model-inference")
	defer span.End()
	span.SetAttributes(attribute.Int("worker.id", workerID), attribute.Int("batch.size", batchSize))
	if failed {
		span.SetStatus(codes.Error, "simulated inference failure")
		request.Emit(ctx, pipeline.Event{Err: fmt.Errorf("worker %d: simulated inference failure", workerID)})
		return
	}
	for tokenIndex := 0; tokenIndex < request.MaxTokens(); tokenIndex++ {
		if !sleepEither(ctx, request.Context(), w.TokenDelay) {
			return
		}
		if !request.Emit(ctx, pipeline.Event{Token: vocabulary[tokenIndex%len(vocabulary)]}) {
			return
		}
	}
	request.Emit(ctx, pipeline.Event{Done: true, Tokens: request.MaxTokens()})
}

func sleepEither(workerCtx, requestCtx context.Context, duration time.Duration) bool {
	if duration <= 0 {
		return workerCtx.Err() == nil && requestCtx.Err() == nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-workerCtx.Done():
		return false
	case <-requestCtx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func sleep(ctx context.Context, duration time.Duration) bool {
	if duration <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
