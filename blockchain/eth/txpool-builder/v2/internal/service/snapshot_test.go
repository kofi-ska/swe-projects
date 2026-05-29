package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"txpool-builder/v2/internal/model"
	rpcx "txpool-builder/v2/internal/rpc"
)

// A healthy snapshot refresh should install one fresh epoch with normalized pending txs.
func TestRefreshSnapshotHappyPath(t *testing.T) {
	cfg := testConfig()
	cfg.OutputDir = t.TempDir()
	cfg.NoWrite = true

	fake := &fakeCaller{
		chainIDs:      []string{"0x1"},
		blockNumbers:  []string{"0x10", "0x10"},
		clientVersion: "geth/v1",
		header:        rpcx.BlockHeader{Number: "0x10", GasLimit: "0x1c9c380", BaseFeePerGas: "0x1"},
		txpool: rawPoolJSON(
			map[string]map[string]any{
				"0xaaa": {
					"0x1": txRaw("0xaaa1", "0x1", "0x5208", "0x1"),
				},
			},
			nil,
		),
	}

	svc := New(cfg, fake, nil)
	if err := svc.refreshSnapshot(context.Background()); err != nil {
		t.Fatalf("refreshSnapshot: %v", err)
	}
	snap := svc.currentSnapshot()
	if snap == nil || snap.SnapshotID == "" {
		t.Fatalf("snapshot not installed")
	}
	if snap.HeadDrift {
		t.Fatalf("unexpected head drift")
	}
	if got := len(snap.PendingBySender["0xaaa"]); got != 1 {
		t.Fatalf("expected one pending tx, got %d", got)
	}
}

// Chain-ID mismatch must stop the service before it captures or builds on the wrong network.
func TestRefreshSnapshotRejectsChainMismatch(t *testing.T) {
	cfg := testConfig()
	cfg.OutputDir = t.TempDir()
	cfg.NoWrite = true
	fake := &fakeCaller{chainIDs: []string{"0x2"}}
	svc := New(cfg, fake, nil)
	err := svc.refreshSnapshot(context.Background())
	if err == nil {
		t.Fatalf("expected chain mismatch")
	}
	var se *model.StartupError
	if !errors.As(err, &se) || se.Code != model.ReasonChainIDMismatch {
		t.Fatalf("expected chain mismatch code, got %v", err)
	}
}

// Strict mode should fail closed when the head moves during capture.
func TestRefreshSnapshotRejectsHeadDriftStrict(t *testing.T) {
	cfg := testConfig()
	cfg.OutputDir = t.TempDir()
	cfg.NoWrite = true
	cfg.AllowHeadDrift = false
	cfg.Strict = true
	fake := &fakeCaller{
		chainIDs:     []string{"0x1"},
		blockNumbers: []string{"0x10", "0x11"},
		header:       rpcx.BlockHeader{Number: "0x10", GasLimit: "0x1c9c380", BaseFeePerGas: "0x1"},
		txpool:       rawPoolJSON(map[string]map[string]any{"0xaaa": {"0x1": txRaw("0xaaa1", "0x1", "0x5208", "0x1")}}, nil),
	}
	svc := New(cfg, fake, nil)
	err := svc.refreshSnapshot(context.Background())
	if err == nil {
		t.Fatalf("expected head drift error")
	}
	var se *model.StartupError
	if !errors.As(err, &se) || se.Code != model.ReasonHeadDrift {
		t.Fatalf("expected head drift code, got %v", err)
	}
}

// A syncing upstream node should be treated as unsafe input, not a usable source of truth.
func TestRefreshSnapshotRejectsSyncingNode(t *testing.T) {
	cfg := testConfig()
	cfg.OutputDir = t.TempDir()
	cfg.NoWrite = true
	fake := &fakeCaller{chainIDs: []string{"0x1"}, syncing: true}
	svc := New(cfg, fake, nil)
	err := svc.refreshSnapshot(context.Background())
	if err == nil {
		t.Fatalf("expected syncing node error")
	}
	var se *model.StartupError
	if !errors.As(err, &se) || se.Code != model.ReasonSyncingNode {
		t.Fatalf("expected syncing code, got %v", err)
	}
}

// Raw snapshot payloads over the configured byte cap must fail before they reach persistence.
func TestRefreshSnapshotRejectsOversizedPayload(t *testing.T) {
	cfg := testConfig()
	cfg.OutputDir = t.TempDir()
	cfg.NoWrite = true
	cfg.MaxSnapshotBytes = 1
	fake := &fakeCaller{
		chainIDs:     []string{"0x1"},
		blockNumbers: []string{"0x10", "0x10"},
		header:       rpcx.BlockHeader{Number: "0x10", GasLimit: "0x1c9c380", BaseFeePerGas: "0x1"},
		txpool:       rawPoolJSON(map[string]map[string]any{"0xaaa": {"0x1": txRaw("0xaaa1", "0x1", "0x5208", "0x1")}}, nil),
	}
	svc := New(cfg, fake, nil)
	err := svc.refreshSnapshot(context.Background())
	if err == nil {
		t.Fatalf("expected oversized payload error")
	}
	if !strings.Contains(err.Error(), "SNAPSHOT_TOO_LARGE") {
		t.Fatalf("expected snapshot too large code, got %v", err)
	}
}

// Malformed txpool_content must surface as a schema error, not a silent fallback.
func TestRefreshSnapshotRejectsMalformedTxPool(t *testing.T) {
	cfg := testConfig()
	cfg.OutputDir = t.TempDir()
	cfg.NoWrite = true
	fake := &fakeCaller{
		chainIDs:     []string{"0x1"},
		blockNumbers: []string{"0x10", "0x10"},
		header:       rpcx.BlockHeader{Number: "0x10", GasLimit: "0x1c9c380", BaseFeePerGas: "0x1"},
		rawByMethod: map[string]json.RawMessage{
			"txpool_content": json.RawMessage(`{"pending":`),
		},
	}
	svc := New(cfg, fake, nil)
	err := svc.refreshSnapshot(context.Background())
	if err == nil {
		t.Fatalf("expected malformed txpool failure")
	}
	var se *model.StartupError
	if !errors.As(err, &se) || se.Code != model.ReasonRPCSchemaError {
		t.Fatalf("expected schema error, got %v", err)
	}
}

// Freshness is a simple time gate: snapshots inside the window are usable, outside are not.
func TestSnapshotFreshEnough(t *testing.T) {
	snap := sampleSnapshot()
	if !snapshotFreshEnough(snap, 10*time.Second) {
		t.Fatalf("expected fresh snapshot")
	}
	snap.CapturedAt = time.Unix(0, 0)
	if snapshotFreshEnough(snap, time.Nanosecond) {
		t.Fatalf("expected stale snapshot")
	}
}
