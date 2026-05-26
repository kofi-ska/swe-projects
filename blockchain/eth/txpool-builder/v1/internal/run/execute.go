package run

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"txpool-builder/v1/internal/config"
	"txpool-builder/v1/internal/model"
	rpcx "txpool-builder/v1/internal/rpc"
)

// Execute keeps the whole build lifecycle visible in one place for auditability.
func Execute(ctx context.Context, c rpcx.Caller, cfg model.Config) (Result, error) {
	start := time.Now()
	cfgDigest := config.Digest(cfg)
	runCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	if !cfg.NoWrite {
		if err := ensureWritable(cfg.OutputPath); err != nil {
			return Result{}, &model.StartupError{Code: model.ReasonArtifactWriteFailed, Stage: "startup", Detail: "candidate output path not writable", Err: err}
		}
		if err := ensureWritable(cfg.TraceOutputPath); err != nil {
			return Result{}, &model.StartupError{Code: model.ReasonTraceWriteFailed, Stage: "startup", Detail: "trace output path not writable", Err: err}
		}
		if err := ensureWritable(cfg.SnapshotOutputPath); err != nil {
			return Result{}, &model.StartupError{Code: model.ReasonSnapshotTooLarge, Stage: "startup", Detail: "snapshot output path not writable", Err: err}
		}
	}

	var (
		startup  model.StartupInfo
		snapshot canonicalSnapshot
		trace    model.DecisionTrace
		err      error
		replay   bool
	)

	if cfg.ReplaySnapshotPath != "" {
		snapshot, err = loadSnapshot(cfg.ReplaySnapshotPath)
		if err != nil {
			return Result{}, &model.StartupError{Code: model.ReasonRPCSchemaError, Stage: "replay", Detail: "failed to load replay snapshot", Err: err}
		}
		snapshot.RawSnapshotPath = cfg.ReplaySnapshotPath
		trace = traceFromSnapshot(snapshot, cfgDigest)
		replay = true
	} else {
		startup, err = startupChecks(runCtx, c, cfg)
		if err != nil {
			return Result{}, err
		}

		snapshot, trace, err = captureSnapshot(runCtx, c, cfg, startup, cfgDigest, start)
		if err != nil {
			return Result{}, err
		}
	}

	candidate, trace, err := buildCandidate(runCtx, cfg, startup, snapshot, trace, cfgDigest, start)
	if err != nil {
		return Result{}, err
	}

	if !cfg.NoWrite {
		snapshot.RawSnapshotPath = cfg.SnapshotOutputPath
		if err := writeSnapshot(cfg.SnapshotOutputPath, snapshot, cfg.MaxRawSnapshotBytes); err != nil {
			return Result{}, &model.StartupError{Code: model.ReasonArtifactWriteFailed, Stage: "persist", Detail: "snapshot write failed", Err: err}
		}
		if err := writeAtomic(cfg.OutputPath, candidate, cfg.MaxArtifactBytes); err != nil {
			return Result{}, &model.StartupError{Code: model.ReasonArtifactWriteFailed, Stage: "persist", Detail: "candidate write failed", Err: err}
		}
		if err := writeAtomic(cfg.TraceOutputPath, trace, cfg.MaxTraceBytes); err != nil {
			return Result{}, &model.StartupError{Code: model.ReasonTraceWriteFailed, Stage: "persist", Detail: "trace write failed", Err: err}
		}
	}

	return Result{
		ConfigDigest: cfgDigest,
		Candidate:    candidate,
		Trace:        trace,
		Snapshot:     snapshot,
		Replay:       replay,
	}, nil
}

// startupChecks fails fast when the upstream chain or node state is wrong.
func startupChecks(ctx context.Context, c rpcx.Caller, cfg model.Config) (model.StartupInfo, error) {
	chainID, err := rpcx.ChainID(ctx, c)
	if err != nil {
		return model.StartupInfo{}, &model.StartupError{Code: model.ReasonRPCUnavailable, Stage: "startup", Detail: "failed to fetch chain id", Err: err}
	}
	if chainID.Cmp(cfg.ChainID) != 0 {
		return model.StartupInfo{}, &model.StartupError{Code: model.ReasonChainIDMismatch, Stage: "startup", Detail: fmt.Sprintf("expected %s got %s", cfg.ChainID.String(), chainID.String())}
	}

	syncing, err := rpcx.Syncing(ctx, c)
	if err != nil {
		return model.StartupInfo{}, &model.StartupError{Code: model.ReasonRPCUnavailable, Stage: "startup", Detail: "failed to fetch syncing status", Err: err}
	}
	if syncing {
		return model.StartupInfo{}, &model.StartupError{Code: model.ReasonSyncingNode, Stage: "startup", Detail: "node is syncing"}
	}

	blockNumber, err := rpcx.BlockNumber(ctx, c)
	if err != nil {
		return model.StartupInfo{}, &model.StartupError{Code: model.ReasonRPCUnavailable, Stage: "startup", Detail: "failed to fetch block number", Err: err}
	}

	clientVersion, _ := rpcx.ClientVersion(ctx, c)
	return model.StartupInfo{
		EndpointLabel: rpcx.EndpointLabel(cfg.RPCURL),
		ChainID:       chainID,
		ClientVersion: clientVersion,
		BlockNumber:   blockNumber,
		Syncing:       syncing,
	}, nil
}

// captureSnapshot turns one upstream epoch into the immutable replay input.
func captureSnapshot(ctx context.Context, c rpcx.Caller, cfg model.Config, startup model.StartupInfo, cfgDigest string, start time.Time) (canonicalSnapshot, model.DecisionTrace, error) {
	trace := model.DecisionTrace{
		SchemaVersion:       1,
		TraceID:             deterministicID("trace", cfgDigest, startup.EndpointLabel, startup.BlockNumber.String()),
		SnapshotID:          "",
		CandidateID:         "",
		PolicyVersion:       cfg.PolicyVersion,
		BinaryVersion:       BinaryVersion,
		ConfigDigest:        cfgDigest,
		SourceEndpointLabel: startup.EndpointLabel,
		ChainID:             startup.ChainID.String(),
		ReasonCodeSummary:   map[string]int{},
		CreatedAt:           time.Now().UTC(),
	}

	headBefore := startup.BlockNumber
	header, err := rpcx.BlockHeaderByNumber(ctx, c, hexNumber(headBefore))
	if err != nil {
		return canonicalSnapshot{}, trace, &model.StartupError{Code: model.ReasonRPCUnavailable, Stage: "snapshot", Detail: "failed to fetch latest block header", Err: err}
	}

	var raw json.RawMessage
	fetchStart := time.Now()
	if err := c.CallContext(ctx, &raw, "txpool_content"); err != nil {
		return canonicalSnapshot{}, trace, &model.StartupError{Code: model.ReasonRPCUnsupportedMethod, Stage: "snapshot", Detail: "txpool_content failed", Err: err}
	}
	fetchMS := time.Since(fetchStart).Milliseconds()
	if cfg.MaxRawSnapshotBytes > 0 && int64(len(raw)) > cfg.MaxRawSnapshotBytes {
		return canonicalSnapshot{}, trace, &model.StartupError{Code: model.ReasonSnapshotTooLarge, Stage: "snapshot", Detail: "raw snapshot exceeds configured maximum"}
	}

	headAfter, err := rpcx.BlockNumber(ctx, c)
	if err != nil {
		return canonicalSnapshot{}, trace, &model.StartupError{Code: model.ReasonRPCUnavailable, Stage: "snapshot", Detail: "failed to fetch post-snapshot block number", Err: err}
	}
	headDrift := headBefore.Cmp(headAfter) != 0
	if headDrift && cfg.Strict && !cfg.AllowHeadDrift {
		return canonicalSnapshot{}, trace, &model.StartupError{Code: model.ReasonHeadDrift, Stage: "snapshot", Detail: "head changed during snapshot fetch"}
	}

	pool, err := decodePool(raw)
	if err != nil {
		return canonicalSnapshot{}, trace, &model.StartupError{Code: model.ReasonRPCSchemaError, Stage: "decode", Detail: "failed to decode txpool_content", Err: err}
	}

	pendingTxs, pendingDecisions, err := normalizePool(pool.Pending, cfg, startup.ChainID, header, "pending")
	if err != nil {
		return canonicalSnapshot{}, trace, err
	}
	queuedTxs := []model.Transaction{}
	queuedDecisions := decisionBuckets{}
	if cfg.IncludeQueued {
		queuedTxs, queuedDecisions, err = normalizePool(pool.Queued, cfg, startup.ChainID, header, "queued")
		if err != nil {
			return canonicalSnapshot{}, trace, err
		}
	}

	snapshot := canonicalSnapshot{
		SchemaVersion:       1,
		SourceEndpointLabel: startup.EndpointLabel,
		CapturedAt:          time.Now().UTC(),
		HeadBefore:          headBefore.String(),
		HeadAfter:           headAfter.String(),
		HeadDrift:           headDrift,
		ChainID:             startup.ChainID.String(),
		ClientVersion:       startup.ClientVersion,
		FetchDurationMS:     fetchMS,
		RawPayloadDigest:    digestBytes(raw),
		RawPendingCount:     countPoolEntries(pool.Pending),
		RawQueuedCount:      countPoolEntries(pool.Queued),
		Pending:             pendingTxs,
		Queued:              queuedTxs,
		RawSnapshotPath:     "",
		RawSnapshotSize:     int64(len(raw)),
	}
	snapshot.SnapshotID = digestCanonical(snapshotFingerprintFrom(snapshot))

	trace.SnapshotID = snapshot.SnapshotID
	trace.DecodeFailures = pendingDecisions.DecodeFailures
	trace.ValidationFailures = pendingDecisions.ValidationFailures
	trace.PolicyRejections = append(trace.PolicyRejections, pendingDecisions.PolicyRejections...)
	trace.PolicyRejections = append(trace.PolicyRejections, queuedDecisions.PolicyRejections...)
	trace.CapacityExclusions = append(trace.CapacityExclusions, pendingDecisions.CapacityExclusions...)
	trace.CapacityExclusions = append(trace.CapacityExclusions, queuedDecisions.CapacityExclusions...)
	trace.Accepted = append(trace.Accepted, pendingDecisions.Accepted...)
	trace.Accepted = append(trace.Accepted, queuedDecisions.Accepted...)
	trace.ReasonCodeSummary = mergeReasonSummary(nil, pendingDecisions)
	trace.ReasonCodeSummary = mergeReasonMaps(trace.ReasonCodeSummary, queuedDecisions)
	trace.FinalSummary = fmt.Sprintf("pending=%d queued=%d accepted=%d rejected=%d", len(pendingTxs), len(queuedTxs), len(trace.Accepted), len(trace.DecodeFailures)+len(trace.ValidationFailures)+len(trace.PolicyRejections)+len(trace.CapacityExclusions))
	trace.SelectionStopReason = "pending-to-selection"
	_ = header
	return snapshot, trace, nil
}
