package service

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"txpool-builder/v2/internal/model"
	rpcx "txpool-builder/v2/internal/rpc"
)

// BenchmarkSubmit measures the admission path because that is the hot path at scale.
func BenchmarkSubmit(b *testing.B) {
	cfg := testConfig()
	cfg.QueueSize = 1024
	svc := newTestService(cfg)
	svc.setSnapshot(sampleSnapshot())
	req := model.BuildRequest{IdempotencyKey: "bench-idem", PriorityClass: "normal", PolicyVersion: "v2"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req.IdempotencyKey = fmt.Sprintf("bench-idem-%d", i%10)
		_, _, _ = svc.Submit(context.Background(), req)
	}
}

// BenchmarkBuildCandidate measures the selection path because it dominates per-job CPU.
func BenchmarkBuildCandidate(b *testing.B) {
	cfg := testConfig()
	cfg.OutputDir = b.TempDir()
	svc := newTestService(cfg)
	snap := sampleSnapshot()
	req := model.BuildRequest{IdempotencyKey: "bench-build", PriorityClass: "normal", PolicyVersion: "v2"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = svc.buildCandidate(context.Background(), req, snap, nil, time.Unix(1, 0))
	}
}

// BenchmarkRefreshSnapshot measures the amortized refresh cost that drives reuse economics.
func BenchmarkRefreshSnapshot(b *testing.B) {
	cfg := testConfig()
	cfg.OutputDir = b.TempDir()
	cfg.NoWrite = true
	fake := &fakeCaller{
		chainIDs:      []string{"0x1"},
		blockNumbers:  []string{"0x10", "0x10"},
		clientVersion: "geth/v1",
		header:        rpcx.BlockHeader{Number: "0x10", GasLimit: "0x1c9c380", BaseFeePerGas: "0x1"},
		txpool:        rawPoolJSON(map[string]map[string]any{"0xaaa": {"0x1": txRaw("0xaaa1", "0x1", "0x5208", "0x1")}}, nil),
	}
	svc := New(cfg, fake, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.refreshSnapshot(context.Background())
	}
}

// BenchmarkWriteJSONAtomic measures persistence cost because artifact writes must stay off the hot path.
func BenchmarkWriteJSONAtomic(b *testing.B) {
	dir := b.TempDir()
	payload := sampleSnapshot()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = writeJSONAtomic(filepath.Join(dir, "payload.json"), payload, 1<<20)
	}
}
