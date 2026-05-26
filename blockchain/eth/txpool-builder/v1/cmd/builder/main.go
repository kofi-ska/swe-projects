package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"txpool-builder/v1/internal/config"
	"txpool-builder/v1/internal/model"
	rpcx "txpool-builder/v1/internal/rpc"
	"txpool-builder/v1/internal/run"

	"github.com/ethereum/go-ethereum/rpc"
)

func main() {
	defer recoverPanic()

	cfg, err := config.Load(os.Args[1:])
	if err != nil {
		fail(err, exitCode(err))
	}

	if cfg.Version {
		fmt.Printf("binary=%s policy=%s\n", run.BinaryVersion, cfg.PolicyVersion)
		return
	}

	if cfg.PrintConfig || cfg.DryRunConfig {
		type printable struct {
			Config model.Config `json:"config"`
			Digest string       `json:"digest"`
			Binary string       `json:"binary"`
			Policy string       `json:"policy"`
		}
		payload := printable{
			Config: cfg,
			Digest: config.Digest(cfg),
			Binary: run.BinaryVersion,
			Policy: cfg.PolicyVersion,
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Println(string(b))
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	runID := run.DeterministicRunID(cfg)
	logger.Info("run_start", "run_id", runID, "binary", run.BinaryVersion, "policy", cfg.PolicyVersion, "config_digest", config.Digest(cfg))

	if cfg.ReplaySnapshotPath != "" {
		logger.Info("replay_mode", "run_id", runID, "snapshot_path", cfg.ReplaySnapshotPath)
	}

	var client *rpc.Client
	if cfg.ReplaySnapshotPath == "" {
		client, err = rpcx.Dial(context.Background(), cfg.RPCURL)
		if err != nil {
			fail(err, exitCode(err))
		}
		defer client.Close()
	}

	start := time.Now()
	result, err := run.Execute(context.Background(), client, cfg)
	if err != nil {
		logger.Error("run_failed", "run_id", runID, "err", err.Error(), "elapsed_ms", time.Since(start).Milliseconds())
		fail(err, exitCode(err))
	}

	var comparison *run.ComparisonResult
	if cfg.CompareCandidatePath != "" {
		comparison, err = run.CompareCandidateArtifact(cfg.CompareCandidatePath, result.Candidate)
		if err != nil {
			fail(err, exitCode(err))
		}
		if !comparison.Match {
			logger.Error("replay_compare_failed", "run_id", runID, "differences", comparison.Differences)
			fail(fmt.Errorf("candidate comparison failed: %v", comparison.Differences), 1)
		}
	}
	logger.Info("run_complete", "run_id", runID, "candidate_id", result.Candidate.CandidateID, "snapshot_id", result.Snapshot.SnapshotID, "tx_count", result.Candidate.TxCount, "total_gas", result.Candidate.TotalGas, "elapsed_ms", time.Since(start).Milliseconds())

	summary := map[string]any{
		"run_id":        runID,
		"candidate_id":  result.Candidate.CandidateID,
		"snapshot_id":   result.Snapshot.SnapshotID,
		"tx_count":      result.Candidate.TxCount,
		"total_gas":     result.Candidate.TotalGas,
		"output":        cfg.OutputPath,
		"trace":         cfg.TraceOutputPath,
		"snapshot":      cfg.SnapshotOutputPath,
		"binary":        run.BinaryVersion,
		"policy":        cfg.PolicyVersion,
		"config_digest": config.Digest(cfg),
		"replay":        result.Replay,
		"no_write":      cfg.NoWrite,
		"comparison":    comparison,
	}
	b, _ := json.Marshal(summary)
	fmt.Println(string(b))
}

func fail(err error, code int) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(code)
}

func recoverPanic() {
	if r := recover(); r != nil {
		fmt.Fprintln(os.Stderr, "PANIC_RECOVERED:", r)
		os.Exit(70)
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if se, ok := err.(*model.StartupError); ok {
		switch se.Code {
		case model.ReasonConfigError:
			return 2
		case model.ReasonRPCUnavailable, model.ReasonRPCTimeout, model.ReasonRPCSchemaError, model.ReasonRPCUnsupportedMethod, model.ReasonChainIDMismatch, model.ReasonSyncingNode, model.ReasonHeadDrift:
			return 3
		case model.ReasonDecodeError, model.ReasonUnsupportedTxType, model.ReasonMissingField, model.ReasonInvalidHex, model.ReasonOverflow, model.ReasonInvalidAddress, model.ReasonInvalidNonce, model.ReasonNonceGap, model.ReasonDuplicateNonce, model.ReasonDuplicateTx, model.ReasonReplacementConflict, model.ReasonInvalidGasLimit, model.ReasonExceedsBlockGas, model.ReasonInvalidFeeModel, model.ReasonInsufficientEffectiveFee, model.ReasonPolicyRejected, model.ReasonCapacityExcluded, model.ReasonStaleSnapshot:
			return 4
		case model.ReasonArtifactWriteFailed, model.ReasonTraceWriteFailed:
			return 5
		case model.ReasonInvariantFailure, model.ReasonPanicRecovered:
			return 6
		}
	}
	if err.Error() == context.DeadlineExceeded.Error() {
		return 3
	}
	return 1
}
