package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"time"

	"txpool-builder/v2/internal/model"
	rpcx "txpool-builder/v2/internal/rpc"
)

type fakeCaller struct {
	mu            sync.Mutex
	chainIDs      []string
	blockNumbers  []string
	syncing       bool
	clientVersion string
	header        rpcx.BlockHeader
	txpool        json.RawMessage
	rawByMethod   map[string]json.RawMessage
	delayByMethod map[string]time.Duration
	errByMethod   map[string]error
	calls         map[string]int
}

// CallContext returns deterministic canned RPC responses so tests can isolate runtime behavior.
func (f *fakeCaller) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	if d := f.delayByMethod[method]; d > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[method]++
	if err, ok := f.errByMethod[method]; ok && err != nil {
		return err
	}
	switch method {
	case "eth_chainId":
		return assignJSON(result, nextValue(&f.chainIDs, "0x1"))
	case "eth_blockNumber":
		return assignJSON(result, nextValue(&f.blockNumbers, "0x0"))
	case "eth_syncing":
		return assignJSON(result, f.syncing)
	case "web3_clientVersion":
		return assignJSON(result, f.clientVersion)
	case "eth_getBlockByNumber":
		return assignJSON(result, f.header)
	case "txpool_content":
		if raw, ok := f.rawByMethod[method]; ok {
			if out, ok := result.(*json.RawMessage); ok {
				*out = append((*out)[:0], raw...)
				return nil
			}
			return assignJSON(result, raw)
		}
		return assignJSON(result, f.txpool)
	default:
		return fmt.Errorf("unsupported method %s", method)
	}
}

// nextValue pops the next canned response and keeps test fixtures deterministic.
func nextValue(values *[]string, fallback string) string {
	if len(*values) == 0 {
		return fallback
	}
	v := (*values)[0]
	*values = (*values)[1:]
	return v
}

// assignJSON copies a value through JSON so test fixtures exercise the same decoding path as production.
func assignJSON(result interface{}, value any) error {
	if result == nil {
		return nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, result)
}

// testConfig creates a small, bounded config that is safe to use in unit tests.
func testConfig() model.Config {
	return model.Config{
		ListenAddr:       ":8080",
		RPCURL:           "http://127.0.0.1:8545",
		OutputDir:        "",
		PolicyVersion:    "v2",
		ChainID:          mustBig("1"),
		RefreshInterval:  10 * time.Millisecond,
		RequestTimeout:   1 * time.Second,
		AdmissionTimeout: 250 * time.Millisecond,
		QueueSize:        8,
		WorkerCount:      2,
		MaxRetainedJobs:  8,
		MaxRetainedSnap:  4,
		MaxArtifactBytes: 1 << 20,
		MaxTraceBytes:    1 << 20,
		MaxSnapshotBytes: 1 << 20,
		MaxSnapshotAge:   10 * time.Second,
		MaxRPCPerRequest: 1,
		MaxRetryAttempts: 2,
		MaxGas:           0,
		MaxTransactions:  4,
		Strict:           true,
	}
}

// mustBig creates a fixed-precision integer for selection and scoring fixtures.
func mustBig(s string) *big.Int {
	n, _ := new(big.Int).SetString(s, 10)
	return n
}

// newTestService builds a service with a fake RPC caller and no external dependencies.
func newTestService(cfg model.Config) *Service {
	return New(cfg, &fakeCaller{}, nil)
}

// sampleSnapshot is a fresh, bounded snapshot fixture with stable ordering.
func sampleSnapshot() *model.Snapshot {
	now := time.Now().UTC()
	return &model.Snapshot{
		SchemaVersion: 1,
		SnapshotID:    "snap-1",
		CapturedAt:    now,
		PolicyVersion: "v2",
		BinaryVersion: BinaryVersion,
		ChainID:       "1",
		BaseFee:       "1",
		GasLimit:      1_000_000,
		MempoolDigest: "digest",
		FreshUntil:    now.Add(10 * time.Second),
		RefreshMS:     1,
		PendingBySender: map[string][]model.Transaction{
			"0xaaa": {
				{Hash: "0xaa1", From: "0xaaa", Nonce: 1, GasLimit: 21_000, Score: mustBig("10")},
				{Hash: "0xaa2", From: "0xaaa", Nonce: 2, GasLimit: 21_000, Score: mustBig("9")},
			},
			"0xbbb": {
				{Hash: "0xbb1", From: "0xbbb", Nonce: 1, GasLimit: 21_000, Score: mustBig("8")},
			},
		},
		QueuedBySender: map[string][]model.Transaction{},
		SourceLabel:    "abc123",
		HeadBefore:     "10",
		HeadAfter:      "10",
	}
}

// nowUTC pins one reference timestamp for deterministic helper tests.
func nowUTC() time.Time {
	return time.Unix(1_700_000_000, 0).UTC()
}

// rawPoolJSON constructs a txpool_content-shaped payload for snapshot tests.
func rawPoolJSON(pending map[string]map[string]any, queued map[string]map[string]any) json.RawMessage {
	doc := map[string]any{
		"pending": pending,
		"queued":  queued,
	}
	b, _ := json.Marshal(doc)
	return b
}

// txRaw creates one raw tx record that matches the schema expected by the decoder.
func txRaw(hash, nonce, gas, gasPrice string) map[string]any {
	return map[string]any{
		"hash":     hash,
		"from":     "0xaaa",
		"gas":      gas,
		"gasPrice": gasPrice,
		"nonce":    nonce,
		"type":     "0x0",
		"value":    "0x0",
		"input":    "0x",
	}
}
