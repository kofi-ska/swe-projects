package relay

import (
	"testing"
	"time"

	"mevrelayv2/internal/backend/local"
	"mevrelayv2/internal/config"
	"mevrelayv2/internal/eventlog"
	"mevrelayv2/internal/model"
	stateMemory "mevrelayv2/internal/state/memory"
)

func TestScoreAdmissionRejectsStaleDeadline(t *testing.T) {
	dir := t.TempDir()
	wal, err := eventlog.Open(dir+"/wal.jsonl", 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer wal.Close()

	svc := New(config.Config{
		RegionID:         "local",
		BackendKind:      "anvil",
		RequestTimeout:   100 * time.Millisecond,
		MinDeadlineSlack: 50 * time.Millisecond,
		ValuePerTx:       1,
		CostPerTx:        0.25,
		CostPerMS:        0.01,
	}, local.New(), nil, stateMemory.New(), wal)

	decision := svc.scoreAdmission(model.BundleRecord{
		Request: model.BundleRequest{
			Txs:          []string{"0x1"},
			BlockNumber:  "0x1",
			MaxTimestamp: time.Now().Add(-time.Second).Unix(),
		},
	})

	if decision.accepted {
		t.Fatalf("expected stale deadline to be rejected: %+v", decision)
	}
	if decision.reason != ErrStaleDeadline.Error() {
		t.Fatalf("expected stale deadline reason, got %q", decision.reason)
	}
}

func TestScoreAdmissionAcceptsHealthyBundle(t *testing.T) {
	dir := t.TempDir()
	wal, err := eventlog.Open(dir+"/wal.jsonl", 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer wal.Close()

	svc := New(config.Config{
		RegionID:         "local",
		BackendKind:      "anvil",
		RequestTimeout:   2 * time.Second,
		MinDeadlineSlack: 50 * time.Millisecond,
		MinNetValue:      0,
		ValuePerTx:       1,
		CostPerTx:        0.25,
		CostPerMS:        0.01,
	}, local.New(), nil, stateMemory.New(), wal)

	decision := svc.scoreAdmission(model.BundleRecord{
		Request: model.BundleRequest{
			Txs:          []string{"0x1", "0x2"},
			BlockNumber:  "0x1",
			MaxTimestamp: time.Now().Add(2 * time.Second).Unix(),
		},
	})

	if !decision.accepted {
		t.Fatalf("expected bundle to be accepted: %+v", decision)
	}
	if decision.priority <= 0 {
		t.Fatalf("expected positive priority: %+v", decision)
	}
}
