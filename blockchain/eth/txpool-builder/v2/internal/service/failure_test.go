package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"txpool-builder/v2/internal/model"
	rpcx "txpool-builder/v2/internal/rpc"
)

// A missing snapshot must fail the job rather than pretending the build succeeded.
func TestProcessJobFailsWhenSnapshotMissing(t *testing.T) {
	cfg := testConfig()
	cfg.OutputDir = t.TempDir()
	svc := newTestService(cfg)
	svc.mu.Lock()
	svc.jobs["job-1"] = &model.JobRecord{
		JobID:          "job-1",
		RequestID:      "req-1",
		IdempotencyKey: "idem-1",
		PriorityClass:  "normal",
		PolicyVersion:  "v2",
		State:          model.JobQueued,
		SnapshotID:     "missing",
		CreatedAt:      nowUTC(),
	}
	svc.mu.Unlock()

	svc.processJob(context.Background(), 0, &jobEnvelope{Request: model.BuildRequest{IdempotencyKey: "idem-1"}, JobID: "job-1"})

	svc.mu.RLock()
	defer svc.mu.RUnlock()
	job := svc.jobs["job-1"]
	if job.State != model.JobFailed || job.ReasonCode != model.ReasonSnapshotTooLarge {
		t.Fatalf("expected failed job for missing snapshot, got state=%s code=%s", job.State, job.ReasonCode)
	}
}

// Retry storms must not let the queue exceed its configured hard cap.
func TestRetryStormDoesNotGrowQueueBeyondCap(t *testing.T) {
	cfg := testConfig()
	cfg.QueueSize = 2
	svc := newTestService(cfg)
	svc.setSnapshot(sampleSnapshot())
	for i := 0; i < 100; i++ {
		_, _, _ = svc.Submit(context.Background(), model.BuildRequest{IdempotencyKey: fmt.Sprintf("storm-%d", i)})
	}
	if len(svc.queue) > cfg.QueueSize {
		t.Fatalf("queue grew beyond cap: %d", len(svc.queue))
	}
}

// Duplicate storms should collapse to a single idempotency record rather than multiplying state.
func TestDuplicateStormCoalescesByIdempotencyKey(t *testing.T) {
	svc := newTestService(testConfig())
	svc.setSnapshot(sampleSnapshot())
	for i := 0; i < 50; i++ {
		_, _, _ = svc.Submit(context.Background(), model.BuildRequest{IdempotencyKey: "same-key"})
	}
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	if len(svc.idempotency) != 1 {
		t.Fatalf("expected one idempotency record, got %d", len(svc.idempotency))
	}
}

// A timeout must surface as an RPC-class failure so the caller can retry safely.
func TestRefreshSnapshotTimeoutPropagates(t *testing.T) {
	cfg := testConfig()
	cfg.OutputDir = t.TempDir()
	cfg.NoWrite = true
	fake := &fakeCaller{
		delayByMethod: map[string]time.Duration{"eth_chainId": 20 * time.Millisecond},
	}
	cfg.RequestTimeout = 1 * time.Millisecond
	svc := New(cfg, fake, nil)
	err := svc.refreshSnapshot(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	var se *model.StartupError
	if !errors.As(err, &se) || se.Code != model.ReasonRPCUnavailable {
		t.Fatalf("expected startup error, got %v", err)
	}
}

// RPC unavailability must be classified explicitly, not hidden behind a generic error.
func TestRefreshSnapshotRPCUnavailable(t *testing.T) {
	cfg := testConfig()
	cfg.OutputDir = t.TempDir()
	cfg.NoWrite = true
	fake := &fakeCaller{
		errByMethod: map[string]error{
			"eth_chainId": errors.New("rpc down"),
		},
	}
	svc := New(cfg, fake, nil)
	err := svc.refreshSnapshot(context.Background())
	if err == nil {
		t.Fatalf("expected rpc error")
	}
	var se *model.StartupError
	if !errors.As(err, &se) || se.Code != model.ReasonRPCUnavailable {
		t.Fatalf("expected rpc unavailable code, got %v", err)
	}
}

// Invalid headers must fail snapshot refresh before selection can see bad bounds.
func TestRefreshSnapshotInvalidBlockHeader(t *testing.T) {
	cfg := testConfig()
	cfg.OutputDir = t.TempDir()
	cfg.NoWrite = true
	fake := &fakeCaller{
		chainIDs:     []string{"0x1"},
		blockNumbers: []string{"0x10", "0x10"},
		header:       rpcx.BlockHeader{Number: "0x10", GasLimit: "bad", BaseFeePerGas: "0x1"},
		txpool:       rawPoolJSON(map[string]map[string]any{"0xaaa": {"0x1": txRaw("0xaaa1", "0x1", "0x5208", "0x1")}}, nil),
	}
	svc := New(cfg, fake, nil)
	err := svc.refreshSnapshot(context.Background())
	if err == nil {
		t.Fatalf("expected invalid header error")
	}
}

// Bad nonce, missing fields, bad gas, and bad fee models must all be classified locally.
func TestDecodeTxClassifiesMalformedInputs(t *testing.T) {
	baseFee := mustBig("1")

	_, decision, err := decodeTx("0xaaa", "bad", json.RawMessage(`{"hash":"0x1","gas":"0x5208","gasPrice":"0x1"}`), baseFee, 1_000_000, "pending")
	if err != nil || decision == nil || decision.PrimaryReason != model.ReasonInvalidNonce {
		t.Fatalf("expected invalid nonce classification, got err=%v decision=%+v", err, decision)
	}

	_, decision, err = decodeTx("0xaaa", "1", json.RawMessage(`{"gas":"0x5208","gasPrice":"0x1"}`), baseFee, 1_000_000, "pending")
	if err != nil || decision == nil || decision.PrimaryReason != model.ReasonMissingField {
		t.Fatalf("expected missing field classification, got err=%v decision=%+v", err, decision)
	}

	_, decision, err = decodeTx("0xaaa", "1", json.RawMessage(`{"hash":"0x1","gas":"0x0","gasPrice":"0x1"}`), baseFee, 1_000_000, "pending")
	if err != nil || decision == nil || decision.PrimaryReason != model.ReasonInvalidGas {
		t.Fatalf("expected invalid gas classification, got err=%v decision=%+v", err, decision)
	}

	_, decision, err = decodeTx("0xaaa", "1", json.RawMessage(`{"hash":"0x1","gas":"0x5208"}`), baseFee, 1_000_000, "pending")
	if err != nil || decision == nil || decision.PrimaryReason != model.ReasonInvalidFeeModel {
		t.Fatalf("expected invalid fee model classification, got err=%v decision=%+v", err, decision)
	}
}
