package pipeline

import (
	"context"
	"time"
)

type Processor interface {
	Process(context.Context, int, []*Request)
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
