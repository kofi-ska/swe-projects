package service

import (
	"math/big"
	"testing"

	"txpool-builder/v2/internal/model"
)

func TestSelectTransactionsDeterministic(t *testing.T) {
	chains := map[string][]model.Transaction{
		"0xbbb": {
			{Hash: "0xbb1", From: "0xbbb", Nonce: 1, GasLimit: 21_000, Score: big.NewInt(30)},
			{Hash: "0xbb2", From: "0xbbb", Nonce: 2, GasLimit: 21_000, Score: big.NewInt(20)},
		},
		"0xaaa": {
			{Hash: "0xaa1", From: "0xaaa", Nonce: 1, GasLimit: 21_000, Score: big.NewInt(40)},
			{Hash: "0xaa2", From: "0xaaa", Nonce: 2, GasLimit: 21_000, Score: big.NewInt(10)},
		},
	}

	selected1, _, _, ranking1, stop1 := selectTransactions(chains, 100_000, 4)
	selected2, _, _, ranking2, stop2 := selectTransactions(chains, 100_000, 4)

	if stop1 != stop2 {
		t.Fatalf("stop reasons differ: %q vs %q", stop1, stop2)
	}
	if len(selected1) != len(selected2) {
		t.Fatalf("selected lengths differ: %d vs %d", len(selected1), len(selected2))
	}
	for i := range selected1 {
		if selected1[i].Hash != selected2[i].Hash {
			t.Fatalf("selected hash mismatch at %d: %s vs %s", i, selected1[i].Hash, selected2[i].Hash)
		}
	}
	for i := range ranking1 {
		if ranking1[i] != ranking2[i] {
			t.Fatalf("ranking mismatch at %d: %s vs %s", i, ranking1[i], ranking2[i])
		}
	}
}

func TestNormalizeSenderChainStopsOnGap(t *testing.T) {
	groups := map[uint64][]model.Transaction{
		1: {{Hash: "0x1", From: "0xaaa", Nonce: 1, GasLimit: 21_000, Score: big.NewInt(10)}},
		3: {{Hash: "0x3", From: "0xaaa", Nonce: 3, GasLimit: 21_000, Score: big.NewInt(5)}},
	}

	selected, decisions := normalizeSenderChain(groups, 1_000_000, "pending")
	if len(selected) != 1 {
		t.Fatalf("expected 1 selected tx, got %d", len(selected))
	}
	if len(decisions) == 0 {
		t.Fatalf("expected a rejection decision for the nonce gap")
	}
	if decisions[len(decisions)-1].PrimaryReason != model.ReasonNonceGap {
		t.Fatalf("expected nonce gap, got %s", decisions[len(decisions)-1].PrimaryReason)
	}
}
