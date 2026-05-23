package relay

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"mevrelayv2/internal/backend/local"
	"mevrelayv2/internal/broker/memory"
	"mevrelayv2/internal/config"
	"mevrelayv2/internal/eventlog"
	"mevrelayv2/internal/scheduler"
	stateMemory "mevrelayv2/internal/state/memory"
)

func TestAssessHealthReflectsQueueAgeAndValue(t *testing.T) {
	dir := t.TempDir()
	wal, err := eventlog.Open(filepath.Join(dir, "wal.jsonl"), 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer wal.Close()

	svc := New(config.Config{
		RegionID:       "local",
		BrokerTopic:    "mev.v2.events",
		BrokerBuffer:   8,
		QueueDepth:     4,
		WorkerCount:    1,
		RequestTimeout: 100 * time.Millisecond,
		MaxQueueAge:    50 * time.Millisecond,
	}, local.New(), memory.New(8), stateMemory.New(), wal)

	now := time.Now().UTC()
	_, accepted, err := svc.queue.Push(scheduler.Item{
		ID:                "bundle-1",
		Priority:          3,
		DeadlineAt:        now.Add(5 * time.Second),
		EnqueuedAt:        now.Add(-time.Second),
		ExpectedValue:     5,
		ExpectedCost:      1,
		ExpectedServiceMS: 50,
	})
	if err != nil || !accepted {
		t.Fatalf("queue push failed: accepted=%v err=%v", accepted, err)
	}

	report := svc.AssessHealth(context.Background())
	if report.State != HealthStateUnsafe {
		t.Fatalf("expected unsafe health, got %+v", report)
	}
	if report.QueueStaleCount == 0 {
		t.Fatalf("expected stale count to be tracked, got %+v", report)
	}
	if report.QueueNetValue <= 0 {
		t.Fatalf("expected queue value to be tracked, got %+v", report)
	}
}
