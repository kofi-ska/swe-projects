package relay

import (
	"context"
	"testing"
	"time"

	"mevrelayv3/internal/config"
)

func TestPolicyMemoryStoreRoundTrip(t *testing.T) {
	store := newPolicyMemoryStore()
	snap := PolicySnapshot{
		Revision:         3,
		Pressure:         0.8,
		Confidence:       0.4,
		MinNetValue:      0.2,
		MinDeadlineSlack: 750 * time.Millisecond,
		MaxQueueAge:      2 * time.Second,
		RetryBackoff:     900 * time.Millisecond,
		QueuePressurePct: 70,
		ConfidenceFloor:  0.6,
	}
	if err := store.Save(context.Background(), "shard-0", snap); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, ok, err := store.Load(context.Background(), "shard-0")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !ok {
		t.Fatalf("expected policy snapshot to exist")
	}
	if got.Revision != snap.Revision || got.Confidence != snap.Confidence {
		t.Fatalf("unexpected snapshot: %+v", got)
	}
}

func TestNewPolicyStoreFallsBackToMemory(t *testing.T) {
	cfg := config.Config{StateKind: "memory"}
	store := newPolicyStore(cfg)
	if store == nil {
		t.Fatalf("expected a policy store")
	}
	if err := store.Save(context.Background(), "shard-0", PolicySnapshot{Revision: 1}); err != nil {
		t.Fatalf("save: %v", err)
	}
}
