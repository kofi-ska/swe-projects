package service

import (
	"context"
	"testing"
	"time"

	"txpool-builder/v2/internal/model"
)

// Nonce gaps must block the rest of a sender chain so later txs never appear valid by accident.
func TestNormalizeSenderChainStopsOnNonceGap(t *testing.T) {
	groups := map[uint64][]model.Transaction{
		1: {{Hash: "0x1", From: "0xaaa", Nonce: 1, GasLimit: 21_000, Score: mustBig("10")}},
		3: {{Hash: "0x3", From: "0xaaa", Nonce: 3, GasLimit: 21_000, Score: mustBig("5")}},
	}
	selected, decisions := normalizeSenderChain(groups, 1_000_000, "pending")
	if len(selected) != 1 {
		t.Fatalf("expected 1 selected tx, got %d", len(selected))
	}
	if len(decisions) == 0 || decisions[len(decisions)-1].PrimaryReason != model.ReasonNonceGap {
		t.Fatalf("expected nonce gap rejection")
	}
}

// The final candidate must stay within both gas and count bounds even when the pool is larger.
func TestBuildCandidateRespectsGasAndCountBounds(t *testing.T) {
	cfg := testConfig()
	cfg.OutputDir = t.TempDir()
	cfg.MaxTransactions = 1
	cfg.MaxGas = 21_000
	svc := newTestService(cfg)
	snap := sampleSnapshot()
	req := model.BuildRequest{IdempotencyKey: "bound-1", PriorityClass: "normal", PolicyVersion: "v2"}

	candidate, trace, err := svc.buildCandidate(context.Background(), req, snap, nil, time.Unix(1, 0))
	if err != nil {
		t.Fatalf("buildCandidate: %v", err)
	}
	if candidate.TxCount != 1 {
		t.Fatalf("expected 1 tx, got %d", candidate.TxCount)
	}
	if candidate.TotalGas > cfg.MaxGas {
		t.Fatalf("gas bound exceeded")
	}
	if trace.SelectionStopReason == "" {
		t.Fatalf("expected stop reason")
	}
}

// An empty pool should produce an empty candidate, not a runtime error.
func TestBuildCandidateEmptyPool(t *testing.T) {
	cfg := testConfig()
	cfg.OutputDir = t.TempDir()
	svc := newTestService(cfg)
	snap := sampleSnapshot()
	snap.PendingBySender = map[string][]model.Transaction{}
	req := model.BuildRequest{IdempotencyKey: "empty", PriorityClass: "normal", PolicyVersion: "v2"}

	candidate, trace, err := svc.buildCandidate(context.Background(), req, snap, nil, time.Unix(1, 0))
	if err != nil {
		t.Fatalf("buildCandidate: %v", err)
	}
	if candidate.TxCount != 0 || len(candidate.SelectedOrder) != 0 {
		t.Fatalf("expected empty candidate")
	}
	if trace.SelectionStopReason != "empty_pool" {
		t.Fatalf("expected empty_pool stop reason, got %s", trace.SelectionStopReason)
	}
}

// When two versions share sender and nonce, the better-scored replacement should survive.
func TestNormalizeSenderChainKeepsBestReplacement(t *testing.T) {
	groups := map[uint64][]model.Transaction{
		1: {
			{Hash: "0xlow", From: "0xaaa", Nonce: 1, GasLimit: 21_000, Score: mustBig("5")},
			{Hash: "0xhigh", From: "0xaaa", Nonce: 1, GasLimit: 21_000, Score: mustBig("8")},
		},
	}
	selected, decisions := normalizeSenderChain(groups, 1_000_000, "pending")
	if len(selected) != 1 || selected[0].Hash != "0xhigh" {
		t.Fatalf("expected best replacement to survive, got %+v", selected)
	}
	if len(decisions) == 0 || decisions[0].PrimaryReason != model.ReasonReplacementConflict {
		t.Fatalf("expected replacement conflict rejection")
	}
}
