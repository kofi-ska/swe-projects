package relay

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mevrelayv2/internal/backend/local"
	"mevrelayv2/internal/broker/memory"
	"mevrelayv2/internal/config"
	"mevrelayv2/internal/eventlog"
	"mevrelayv2/internal/model"
	stateMemory "mevrelayv2/internal/state/memory"
)

func TestSubmitAndHealth(t *testing.T) {
	dir := t.TempDir()
	wal, err := eventlog.Open(filepath.Join(dir, "wal.jsonl"), 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer wal.Close()

	svc := New(config.Config{
		HTTPAddr:             ":0",
		DataDir:              dir,
		RegionID:             "local",
		BrokerKind:           "memory",
		BrokerTopic:          "mev.v2.events",
		BrokerBuffer:         16,
		QueueDepth:           8,
		WorkerCount:          1,
		MaxRetries:           2,
		RetryBackoff:         10 * time.Millisecond,
		MaxPayloadBytes:      256 * 1024,
		MaxInFlightPerClient: 4,
		RequestTimeout:       100 * time.Millisecond,
	}, local.New(), memory.New(8), stateMemory.New(), wal)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx)
	defer svc.Stop()

	rec, err := svc.Submit(ctx, model.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "eth_sendBundle",
		Params: []model.BundleRequest{{
			Txs:         []string{"0x1", "0x2"},
			BlockNumber: "0x1",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.State != model.StateQueued && rec.State != model.StateReceived {
		t.Fatalf("unexpected state: %s", rec.State)
	}

	time.Sleep(50 * time.Millisecond)

	got, ok, err := svc.Get(ctx, rec.ID)
	if err != nil || !ok {
		t.Fatalf("get failed: ok=%v err=%v", ok, err)
	}
	if got.State == model.StateReceived || got.State == model.StateValidated || got.State == model.StateQueued {
		t.Fatalf("bundle did not advance: %s", got.State)
	}

	report := svc.AssessHealth(ctx)
	if report.State == HealthStateUnsafe {
		t.Fatalf("unexpected unsafe health: %+v", report)
	}
}

func TestDuplicateRejected(t *testing.T) {
	dir := t.TempDir()
	wal, err := eventlog.Open(filepath.Join(dir, "wal.jsonl"), 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer wal.Close()

	svc := New(config.Config{
		RegionID:             "local",
		BrokerTopic:          "mev.v2.events",
		BrokerBuffer:         8,
		QueueDepth:           8,
		WorkerCount:          1,
		MaxRetries:           1,
		RetryBackoff:         10 * time.Millisecond,
		MaxPayloadBytes:      256 * 1024,
		MaxInFlightPerClient: 2,
		RequestTimeout:       100 * time.Millisecond,
	}, local.New(), memory.New(8), stateMemory.New(), wal)

	req := model.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      7,
		Method:  "eth_sendBundle",
		Params:  []model.BundleRequest{{Txs: []string{"0x1"}, BlockNumber: "0x1"}},
	}
	if _, err := svc.Submit(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Submit(context.Background(), req); err == nil {
		t.Fatal("expected duplicate rejection")
	}
}

func TestWALWritten(t *testing.T) {
	dir := t.TempDir()
	walPath := filepath.Join(dir, "wal.jsonl")
	wal, err := eventlog.Open(walPath, 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer wal.Close()

	if err := wal.Append(context.Background(), "event", map[string]string{"hello": "world"}); err != nil {
		t.Fatal(err)
	}
	entries, err := wal.ReadAll(16)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if err := os.Remove(walPath); err != nil {
		t.Fatal(err)
	}
}
