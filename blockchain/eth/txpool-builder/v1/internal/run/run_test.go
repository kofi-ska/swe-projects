package run

import (
	"context"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"txpool-builder/v1/internal/model"
	rpcx "txpool-builder/v1/internal/rpc"
)

type fakeCaller struct {
	responses map[string]any
}

func (f fakeCaller) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	_ = ctx
	_ = args
	v, ok := f.responses[method]
	if !ok {
		return nil
	}
	switch out := result.(type) {
	case *string:
		*out = v.(string)
	case *json.RawMessage:
		*out = v.(json.RawMessage)
	case *rpcx.BlockHeader:
		*out = v.(rpcx.BlockHeader)
	default:
		return nil
	}
	return nil
}

type driftCaller struct {
	base    fakeCaller
	counter int
}

func (d *driftCaller) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	_ = ctx
	_ = args
	if method == "eth_blockNumber" {
		d.counter++
		if d.counter == 1 {
			if out, ok := result.(*string); ok {
				*out = "0x10"
			}
			return nil
		}
		if out, ok := result.(*string); ok {
			*out = "0x11"
		}
		return nil
	}
	return d.base.CallContext(ctx, result, method, args...)
}

func TestNormalizePoolRejectsReplacementConflict(t *testing.T) {
	cfg := baseConfig(t)
	raw := mustPoolJSON(map[string]map[string]map[string]any{
		"pending": {
			"0x1111111111111111111111111111111111111111": {
				"0x0":  txJSON("0xaaaa000000000000000000000000000000000000000000000000000000000001", "0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222", "0x0", "0x5208", "0x5", "0x0", "0x"),
				"0x0b": txJSON("0xaaaa000000000000000000000000000000000000000000000000000000000002", "0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222", "0x0", "0x5208", "0x9", "0x0", "0x"),
			},
		},
	})

	pool, err := decodePool(raw)
	if err != nil {
		t.Fatal(err)
	}

	header := rpcx.BlockHeader{Number: "0x10", GasLimit: "0x100000", BaseFeePerGas: "0x1"}
	txs, decisions, err := normalizePool(pool.Pending, cfg, big.NewInt(1), header, "pending")
	if err != nil {
		t.Fatal(err)
	}
	if len(txs) != 1 {
		t.Fatalf("expected 1 tx, got %d", len(txs))
	}
	if len(decisions.PolicyRejections) == 0 {
		t.Fatal("expected replacement conflict rejection")
	}
	if decisions.PolicyRejections[0].PrimaryReason != model.ReasonReplacementConflict {
		t.Fatalf("unexpected reason: %s", decisions.PolicyRejections[0].PrimaryReason)
	}
}

func TestGreedySelectDeterministic(t *testing.T) {
	cfg := baseConfig(t)
	cfg.MaxTransactions = 4
	cfg.MaxGas = 1_000_000

	txs := []model.Transaction{
		txModel("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000001", "0x1111111111111111111111111111111111111111", 0, 5, 21000),
		txModel("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000002", "0x2222222222222222222222222222222222222222", 0, 4, 21000),
		txModel("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000003", "0x1111111111111111111111111111111111111111", 1, 10, 21000),
		txModel("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000004", "0x2222222222222222222222222222222222222222", 1, 1, 21000),
	}

	selected, res, stopReason, order, err := greedySelect(txs, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if stopReason != "selection_complete" {
		t.Fatalf("unexpected stop reason: %s", stopReason)
	}
	if len(selected) != 4 {
		t.Fatalf("expected 4 selected, got %d", len(selected))
	}
	want := []string{
		"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000001",
		"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000003",
		"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000002",
		"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000004",
	}
	got := hashesOf(selected)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected order: %v", got)
	}
	if len(res.Accepted) != 4 {
		t.Fatalf("expected 4 accepted decisions, got %d", len(res.Accepted))
	}
	if len(order) == 0 {
		t.Fatal("expected ranking order")
	}
}

func TestExecuteWritesArtifacts(t *testing.T) {
	dir := t.TempDir()
	cfg := baseConfig(t)
	cfg.OutputPath = filepath.Join(dir, "candidate.json")
	cfg.TraceOutputPath = filepath.Join(dir, "trace.json")
	cfg.SnapshotOutputPath = filepath.Join(dir, "snapshot.json")

	raw := mustPoolJSON(map[string]map[string]map[string]any{
		"pending": {
			"0x1111111111111111111111111111111111111111": {
				"0x0": txJSON("0xaaaa000000000000000000000000000000000000000000000000000000000001", "0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222", "0x0", "0x5208", "0x5", "0x0", "0x"),
			},
		},
	})
	fake := fakeCaller{responses: map[string]any{
		"eth_chainId":          "0x1",
		"eth_blockNumber":      "0x10",
		"eth_syncing":          json.RawMessage("false"),
		"web3_clientVersion":   "Geth/v1",
		"eth_getBlockByNumber": rpcx.BlockHeader{Number: "0x10", GasLimit: "0x100000", BaseFeePerGas: "0x1"},
		"txpool_content":       raw,
	}}

	res, err := Execute(context.Background(), fake, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Candidate.TxCount != 1 {
		t.Fatalf("expected 1 tx, got %d", res.Candidate.TxCount)
	}
	if _, err := os.Stat(cfg.OutputPath); err != nil {
		t.Fatalf("candidate artifact not written: %v", err)
	}
	if _, err := os.Stat(cfg.TraceOutputPath); err != nil {
		t.Fatalf("trace artifact not written: %v", err)
	}
	if _, err := os.Stat(cfg.SnapshotOutputPath); err != nil {
		t.Fatalf("snapshot artifact not written: %v", err)
	}
}

func TestExecuteReplaySnapshotNoWrite(t *testing.T) {
	dir := t.TempDir()
	snapshot := canonicalSnapshot{
		SchemaVersion:       1,
		SnapshotID:          "snap-1",
		SourceEndpointLabel: "replay",
		CapturedAt:          time.Now().UTC(),
		HeadBefore:          "16",
		HeadAfter:           "16",
		HeadDrift:           false,
		ChainID:             "1",
		FetchDurationMS:     1,
		RawPayloadDigest:    "abc",
		RawPendingCount:     1,
		RawQueuedCount:      0,
		Pending: []model.Transaction{
			txModel("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000005", "0x1111111111111111111111111111111111111111", 0, 5, 21000),
		},
		RawSnapshotPath: filepath.Join(dir, "snapshot.json"),
		RawSnapshotSize: 1,
	}
	writeOrFatal(t, snapshot.RawSnapshotPath, snapshot)

	cfg := baseConfig(t)
	cfg.RPCURL = ""
	cfg.OutputPath = ""
	cfg.TraceOutputPath = ""
	cfg.SnapshotOutputPath = ""
	cfg.ReplaySnapshotPath = snapshot.RawSnapshotPath
	cfg.NoWrite = true
	cfg.ChainID = nil

	res, err := Execute(context.Background(), nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Replay {
		t.Fatal("expected replay mode")
	}
	if res.Candidate.TxCount != 1 {
		t.Fatalf("expected 1 tx, got %d", res.Candidate.TxCount)
	}
}

func TestNormalizePoolDeterministicAcrossRepeatedRuns(t *testing.T) {
	cfg := baseConfig(t)
	raw := mustPoolJSON(map[string]map[string]map[string]any{
		"pending": {
			"0x3333333333333333333333333333333333333333": {
				"0x2": txJSON("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000020", "0x3333333333333333333333333333333333333333", "0x2222222222222222222222222222222222222222", "0x2", "0x5208", "0x6", "0x0", "0x"),
				"0x1": txJSON("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000021", "0x3333333333333333333333333333333333333333", "0x2222222222222222222222222222222222222222", "0x1", "0x5208", "0x7", "0x0", "0x"),
			},
			"0x1111111111111111111111111111111111111111": {
				"0x0": txJSON("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000022", "0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222", "0x0", "0x5208", "0x5", "0x0", "0x"),
			},
		},
	})
	pool, err := decodePool(raw)
	if err != nil {
		t.Fatal(err)
	}
	header := rpcx.BlockHeader{Number: "0x10", GasLimit: "0x100000", BaseFeePerGas: "0x1"}

	var want string
	for i := 0; i < 50; i++ {
		txs, _, err := normalizePool(pool.Pending, cfg, big.NewInt(1), header, "pending")
		if err != nil {
			t.Fatal(err)
		}
		got := strings.Join(hashesOf(txs), ",")
		if i == 0 {
			want = got
			continue
		}
		if got != want {
			t.Fatalf("non-deterministic tx order: %q != %q", got, want)
		}
	}
}

func TestCompareCandidateArtifactMatches(t *testing.T) {
	dir := t.TempDir()
	candidate := model.BlockCandidate{
		SchemaVersion:            1,
		CandidateID:              "cand-1",
		SnapshotID:               "snap-1",
		PolicyVersion:            "v1",
		BinaryVersion:            BinaryVersion,
		ConfigDigest:             "cfg-1",
		SourceEndpointLabel:      "src",
		ChainID:                  "1",
		SelectedTxHashes:         []string{"0x1"},
		SelectedOrder:            []string{"0x1"},
		TxCount:                  1,
		TotalGas:                 21000,
		EstimatedPriorityRevenue: "5",
		RejectedCount:            0,
		RejectionSummary:         map[string]int{},
		SelectionStopReason:      "selection_complete",
		BuildDurationMS:          1,
		TraceRef:                 "trace-1",
		IsExecutableBlock:        false,
	}
	path := filepath.Join(dir, "candidate.json")
	b, err := json.Marshal(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := CompareCandidateArtifact(path, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Match {
		t.Fatalf("expected match, got %+v", res)
	}
}

func TestSnapshotFingerprintIgnoresTiming(t *testing.T) {
	a := canonicalSnapshot{
		SchemaVersion:       1,
		SnapshotID:          "snap-a",
		SourceEndpointLabel: "src",
		CapturedAt:          time.Now().UTC(),
		HeadBefore:          "16",
		HeadAfter:           "16",
		HeadDrift:           false,
		ChainID:             "1",
		ClientVersion:       "v1",
		RawPayloadDigest:    "digest",
		RawPendingCount:     1,
		RawQueuedCount:      0,
		Pending: []model.Transaction{
			txModel("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000030", "0x1111111111111111111111111111111111111111", 0, 5, 21000),
		},
		RawSnapshotSize: 123,
	}
	b := a
	b.SnapshotID = "snap-b"
	b.CapturedAt = a.CapturedAt.Add(10 * time.Minute)
	b.FetchDurationMS = 999
	if digestCanonical(snapshotFingerprintFrom(a)) != digestCanonical(snapshotFingerprintFrom(b)) {
		t.Fatal("expected fingerprint to ignore timing metadata")
	}
}

func TestLoadSnapshotRejectsInvalidSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"snapshot_id":"","source_endpoint_label":"","chain_id":"","raw_payload_digest":""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSnapshot(path); err == nil {
		t.Fatal("expected invalid snapshot failure")
	}
}

func TestDecodeTxRejectsZeroGas(t *testing.T) {
	raw := mustPoolJSON(txJSON("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000006", "0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222", "0x0", "0x0", "0x5", "0x0", "0x"))
	_, decision, err := decodeTx("0x1111111111111111111111111111111111111111", "0x0", raw, big.NewInt(1), 0x100000, "pending")
	if err == nil {
		t.Fatal("expected decode failure")
	}
	if decision == nil || decision.PrimaryReason != model.ReasonInvalidGasLimit {
		t.Fatal("expected zero gas validation failure")
	}
}

func TestDecodeTxRejectsUnsupportedType(t *testing.T) {
	raw := mustPoolJSON(map[string]any{
		"hash":     "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000009",
		"from":     "0x1111111111111111111111111111111111111111",
		"to":       "0x2222222222222222222222222222222222222222",
		"nonce":    "0x0",
		"type":     "0x3",
		"gas":      "0x5208",
		"gasPrice": "0x5",
		"value":    "0x0",
		"input":    "0x",
	})
	_, decision, err := decodeTx("0x1111111111111111111111111111111111111111", "0x0", raw, big.NewInt(1), 0x100000, "pending")
	if err == nil {
		t.Fatal("expected unsupported type failure")
	}
	if decision == nil || decision.PrimaryReason != model.ReasonUnsupportedTxType {
		t.Fatalf("expected unsupported type reason, got %+v", decision)
	}
}

func TestExecuteFailsOnHeadDriftWhenStrict(t *testing.T) {
	cfg := baseConfig(t)
	raw := mustPoolJSON(map[string]map[string]map[string]any{
		"pending": {
			"0x1111111111111111111111111111111111111111": {
				"0x0": txJSON("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000010", "0x1111111111111111111111111111111111111111", "0x2222222222222222222222222222222222222222", "0x0", "0x5208", "0x5", "0x0", "0x"),
			},
		},
	})
	fake := fakeCaller{responses: map[string]any{
		"eth_chainId":          "0x1",
		"eth_blockNumber":      "0x10",
		"eth_syncing":          json.RawMessage("false"),
		"web3_clientVersion":   "Geth/v1",
		"eth_getBlockByNumber": rpcx.BlockHeader{Number: "0x10", GasLimit: "0x100000", BaseFeePerGas: "0x1"},
		"txpool_content":       raw,
	}}
	_, err := Execute(context.Background(), &driftCaller{base: fake}, cfg)
	if err == nil {
		t.Fatal("expected head drift failure")
	}
}

func TestGreedySelectHonorsGasCap(t *testing.T) {
	cfg := baseConfig(t)
	cfg.MaxTransactions = 10
	cfg.MaxGas = 21000
	txs := []model.Transaction{
		txModel("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000007", "0x1111111111111111111111111111111111111111", 0, 5, 21000),
		txModel("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa000000000000000000000008", "0x2222222222222222222222222222222222222222", 0, 4, 21000),
	}
	selected, res, _, _, err := greedySelect(txs, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 1 {
		t.Fatalf("expected 1 selected, got %d", len(selected))
	}
	if len(res.CapacityExclusions) == 0 {
		t.Fatal("expected capacity exclusion")
	}
}

func baseConfig(t *testing.T) model.Config {
	t.Helper()
	dir := t.TempDir()
	return model.Config{
		RPCURL:                "http://geth:8545",
		OutputPath:            filepath.Join(dir, "candidate.json"),
		TraceOutputPath:       filepath.Join(dir, "trace.json"),
		SnapshotOutputPath:    filepath.Join(dir, "snapshot.json"),
		Timeout:               5 * time.Second,
		MaxTransactions:       10,
		MaxGas:                1_000_000,
		MaxSnapshotTxs:        100,
		MaxRawSnapshotBytes:   10_000_000,
		MaxArtifactBytes:      10_000_000,
		MaxTraceBytes:         10_000_000,
		PolicyVersion:         "v1",
		ChainID:               big.NewInt(1),
		Strict:                true,
		RejectOnPartialDecode: true,
		AllowHeadDrift:        false,
		IncludeQueued:         false,
	}
}

func txJSON(hash, from, to, nonce, gas, gasPrice, value, input string) map[string]any {
	return map[string]any{
		"hash":     hash,
		"from":     from,
		"to":       to,
		"nonce":    nonce,
		"type":     "0x0",
		"gas":      gas,
		"gasPrice": gasPrice,
		"value":    value,
		"input":    input,
	}
}

func txModel(hash, from string, nonce uint64, gasPrice uint64, gasLimit uint64) model.Transaction {
	gp := new(big.Int).SetUint64(gasPrice)
	score := new(big.Int).Mul(gp, new(big.Int).SetUint64(gasLimit))
	return model.Transaction{
		Hash:                 hash,
		From:                 from,
		Nonce:                nonce,
		TxType:               0,
		GasLimit:             gasLimit,
		GasPrice:             gp,
		Value:                big.NewInt(0),
		InputLen:             0,
		IntrinsicGas:         gasLimit,
		EffectiveGasPrice:    gp,
		EffectivePriorityFee: gp,
		Score:                score,
	}
}

func writeOrFatal(t *testing.T, path string, value any) {
	t.Helper()
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustPoolJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
