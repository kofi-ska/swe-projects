package config

import (
	"math/big"
	"testing"
	"time"

	"txpool-builder/v2/internal/model"
)

func TestValidateRejectsMissingChainID(t *testing.T) {
	cfg := model.Config{
		ListenAddr:       ":8080",
		RPCURL:           "http://127.0.0.1:8545",
		OutputDir:        t.TempDir(),
		PolicyVersion:    "v2",
		RefreshInterval:  time.Second,
		RequestTimeout:   time.Second,
		AdmissionTimeout: time.Millisecond,
		QueueSize:        1,
		WorkerCount:      1,
		MaxRetainedJobs:  1,
		MaxRetainedSnap:  1,
		MaxArtifactBytes: 1,
		MaxTraceBytes:    1,
		MaxSnapshotBytes: 1,
		MaxSnapshotAge:   time.Second,
		MaxRPCPerRequest: 1,
		MaxRetryAttempts: 0,
		MaxTransactions:  1,
	}
	if err := Validate(cfg); err == nil {
		t.Fatalf("expected validation failure")
	}
}

func TestDigestStable(t *testing.T) {
	cfg := model.Config{
		ListenAddr:       ":8080",
		RPCURL:           "http://127.0.0.1:8545",
		OutputDir:        t.TempDir(),
		PolicyVersion:    "v2",
		ChainID:          bigOne(),
		RefreshInterval:  time.Second,
		RequestTimeout:   time.Second,
		AdmissionTimeout: time.Millisecond,
		QueueSize:        1,
		WorkerCount:      1,
		MaxRetainedJobs:  1,
		MaxRetainedSnap:  1,
		MaxArtifactBytes: 1,
		MaxTraceBytes:    1,
		MaxSnapshotBytes: 1,
		MaxSnapshotAge:   time.Second,
		MaxRPCPerRequest: 1,
		MaxRetryAttempts: 0,
		MaxTransactions:  1,
	}
	other := cfg
	if Digest(cfg) != Digest(other) {
		t.Fatalf("digest should be stable")
	}
}

func TestLoadParsesCLIAndEnv(t *testing.T) {
	t.Setenv("TXPOOL_BUILDER_RPC_URL", "http://127.0.0.1:8555")
	t.Setenv("TXPOOL_BUILDER_CHAIN_ID", "0x1")
	t.Setenv("TXPOOL_BUILDER_OUTPUT_DIR", t.TempDir())
	cfg, err := Load([]string{"--listen", ":9090", "--policy-version", "v2", "--print-config"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ListenAddr != ":9090" || cfg.RPCURL != "http://127.0.0.1:8555" {
		t.Fatalf("unexpected load result: %+v", cfg)
	}
	if !cfg.PrintConfig {
		t.Fatalf("expected print-config flag")
	}
}

func TestLoadRejectsInvalidChainID(t *testing.T) {
	t.Setenv("TXPOOL_BUILDER_RPC_URL", "http://127.0.0.1:8555")
	t.Setenv("TXPOOL_BUILDER_CHAIN_ID", "bad-value")
	t.Setenv("TXPOOL_BUILDER_OUTPUT_DIR", t.TempDir())
	_, err := Load([]string{})
	if err == nil {
		t.Fatalf("expected invalid chain id error")
	}
}

func bigOne() *big.Int {
	return big.NewInt(1)
}
