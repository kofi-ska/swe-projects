package scheduler

import (
	"context"
	"testing"
	"time"
)

func TestQueueOrdersByPriority(t *testing.T) {
	q := New(4)
	now := time.Now().UTC()

	if _, accepted, err := q.Push(Item{ID: "low", Priority: 1, DeadlineAt: now.Add(time.Second), EnqueuedAt: now}); err != nil || !accepted {
		t.Fatalf("push low failed: accepted=%v err=%v", accepted, err)
	}
	if _, accepted, err := q.Push(Item{ID: "high", Priority: 10, DeadlineAt: now.Add(time.Second), EnqueuedAt: now.Add(time.Millisecond)}); err != nil || !accepted {
		t.Fatalf("push high failed: accepted=%v err=%v", accepted, err)
	}

	item, ok, err := q.Pop(context.Background())
	if err != nil || !ok {
		t.Fatalf("pop failed: ok=%v err=%v", ok, err)
	}
	if item.ID != "high" {
		t.Fatalf("expected high first, got %s", item.ID)
	}
}

func TestQueueEvictsWorst(t *testing.T) {
	q := New(1)
	now := time.Now().UTC()

	if _, accepted, err := q.Push(Item{ID: "low", Priority: 1, DeadlineAt: now.Add(time.Second), EnqueuedAt: now}); err != nil || !accepted {
		t.Fatalf("push low failed: accepted=%v err=%v", accepted, err)
	}
	evicted, accepted, err := q.Push(Item{ID: "high", Priority: 2, DeadlineAt: now.Add(2 * time.Second), EnqueuedAt: now.Add(time.Millisecond)})
	if err != nil || !accepted {
		t.Fatalf("push high failed: accepted=%v err=%v", accepted, err)
	}
	if evicted == nil || evicted.ID != "low" {
		t.Fatalf("expected low evicted, got %#v", evicted)
	}
	item, ok, err := q.Pop(context.Background())
	if err != nil || !ok {
		t.Fatalf("pop failed: ok=%v err=%v", ok, err)
	}
	if item.ID != "high" {
		t.Fatalf("expected high after eviction, got %s", item.ID)
	}
}
