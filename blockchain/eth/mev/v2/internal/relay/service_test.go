package relay

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mevrelayv2/internal/backend/local"
	"mevrelayv2/internal/broker"
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

func TestRetryBudgetExact(t *testing.T) {
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
		MaxRetries:           1,
		RetryBackoff:         10 * time.Millisecond,
		MaxPayloadBytes:      256 * 1024,
		MaxInFlightPerClient: 2,
		RequestTimeout:       100 * time.Millisecond,
		ValuePerTx:           1,
		CostPerTx:            0.25,
		CostPerMS:            0.01,
	}, local.New(), memory.New(8), stateMemory.New(), wal)

	ctx := context.Background()
	rec := model.BundleRecord{
		ID:                "bundle-retry",
		BundleHash:        "0xaaa",
		Request:           model.BundleRequest{Txs: []string{"0x1", "0x2", "0x3", "0x4", "0x5"}, BlockNumber: "0x1"},
		ClientID:          "anonymous",
		RegionID:          "local",
		State:             model.StateRetryPending,
		Reason:            "retry pending",
		Version:           1,
		Sequence:          1,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
		DeadlineAt:        time.Now().UTC().Add(time.Second),
		ExpectedValue:     5,
		ExpectedCost:      1,
		ExpectedServiceMS: 25,
		Priority:          1,
		RetryCount:        1,
	}
	if _, err := svc.state.CreateBundle(ctx, rec); err != nil {
		t.Fatal(err)
	}
	svc.processRetry(ctx, rec.ID)
	got, ok, err := svc.Get(ctx, rec.ID)
	if err != nil || !ok {
		t.Fatalf("get failed: ok=%v err=%v", ok, err)
	}
	if got.RetryCount != 1 {
		t.Fatalf("expected exact max retries, got %d", got.RetryCount)
	}
	if got.State != model.StateCompleted || got.Terminal != string(model.StateDeadLetter) {
		t.Fatalf("unexpected terminal state: %+v", got)
	}
}

func TestInflightReleasedOnLateTerminalFailure(t *testing.T) {
	dir := t.TempDir()
	wal, err := eventlog.Open(filepath.Join(dir, "wal.jsonl"), 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer wal.Close()

	broker := &countingBroker{failAt: 9}
	st := stateMemory.New()
	svc := New(config.Config{
		RegionID:             "local",
		BrokerTopic:          "mev.v2.events",
		BrokerBuffer:         8,
		QueueDepth:           8,
		WorkerCount:          1,
		MaxRetries:           0,
		RetryBackoff:         10 * time.Millisecond,
		MaxPayloadBytes:      256 * 1024,
		MaxInFlightPerClient: 2,
		RequestTimeout:       100 * time.Millisecond,
		ValuePerTx:           1,
		CostPerTx:            0.25,
		CostPerMS:            0.01,
	}, local.New(), broker, st, wal)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx)
	defer svc.Stop()

	req := model.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      100,
		Method:  "eth_sendBundle",
		Params:  []model.BundleRequest{{Txs: []string{"0x1", "0x2"}, BlockNumber: "0x1"}},
	}
	rec, err := svc.Submit(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	inflight, err := st.GetInflight(ctx, "anonymous")
	if err != nil {
		t.Fatal(err)
	}
	if inflight != 0 {
		t.Fatalf("expected inflight release after late failure, got %d", inflight)
	}
	got, ok, err := svc.Get(ctx, rec.ID)
	if err != nil || !ok {
		t.Fatalf("get failed: ok=%v err=%v", ok, err)
	}
	if got.State != model.StateCompleted && got.State != model.StateDeadLetter {
		t.Fatalf("unexpected terminal state: %+v", got)
	}
}

type countingBroker struct {
	failAt int
	count  int
}

func (b *countingBroker) Publish(context.Context, broker.Message) error {
	b.count++
	if b.failAt > 0 && b.count == b.failAt {
		return errors.New("publish failed")
	}
	return nil
}
func (b *countingBroker) Subscribe(context.Context, string, string) (broker.Consumer, error) {
	return nil, errors.New("not implemented")
}
func (b *countingBroker) Ping(context.Context) error { return nil }
func (b *countingBroker) Close() error               { return nil }
