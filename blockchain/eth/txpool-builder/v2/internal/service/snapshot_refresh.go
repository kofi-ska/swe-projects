package service

import (
	"context"
	"fmt"
	"time"

	"txpool-builder/v2/internal/model"
	rpcx "txpool-builder/v2/internal/rpc"
)

// refreshSnapshot keeps one active epoch so many jobs can amortize one capture.
func (s *Service) refreshSnapshot(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, s.cfg.RequestTimeout)
	defer cancel()

	start := time.Now()
	chainID, err := rpcx.ChainID(ctx, s.rpc)
	if err != nil {
		return &model.StartupError{Code: model.ReasonRPCUnavailable, Stage: "snapshot", Detail: "failed to fetch chain id", Err: err}
	}
	if chainID.Cmp(s.cfg.ChainID) != 0 {
		return &model.StartupError{Code: model.ReasonChainIDMismatch, Stage: "snapshot", Detail: fmt.Sprintf("expected %s got %s", s.cfg.ChainID.String(), chainID.String())}
	}

	syncing, err := rpcx.Syncing(ctx, s.rpc)
	if err != nil {
		return &model.StartupError{Code: model.ReasonRPCUnavailable, Stage: "snapshot", Detail: "failed to fetch syncing status", Err: err}
	}
	if syncing {
		return &model.StartupError{Code: model.ReasonSyncingNode, Stage: "snapshot", Detail: "node is syncing"}
	}

	headBefore, err := rpcx.BlockNumber(ctx, s.rpc)
	if err != nil {
		return &model.StartupError{Code: model.ReasonRPCUnavailable, Stage: "snapshot", Detail: "failed to fetch block number", Err: err}
	}
	header, err := rpcx.BlockHeaderByNumber(ctx, s.rpc, hexNumber(headBefore))
	if err != nil {
		return &model.StartupError{Code: model.ReasonRPCUnavailable, Stage: "snapshot", Detail: "failed to fetch block header", Err: err}
	}

	raw, err := rpcx.TxPoolContent(ctx, s.rpc)
	if err != nil {
		return &model.StartupError{Code: model.ReasonRPCUnsupportedMethod, Stage: "snapshot", Detail: "txpool_content failed", Err: err}
	}
	if s.cfg.MaxSnapshotBytes > 0 && int64(len(raw)) > s.cfg.MaxSnapshotBytes {
		return &model.StartupError{Code: model.ReasonSnapshotTooLarge, Stage: "snapshot", Detail: "raw snapshot exceeds configured maximum"}
	}

	headAfter, err := rpcx.BlockNumber(ctx, s.rpc)
	if err != nil {
		return &model.StartupError{Code: model.ReasonRPCUnavailable, Stage: "snapshot", Detail: "failed to fetch post-snapshot block number", Err: err}
	}
	headDrift := headBefore.Cmp(headAfter) != 0
	if headDrift && s.cfg.Strict && !s.cfg.AllowHeadDrift {
		return &model.StartupError{Code: model.ReasonHeadDrift, Stage: "snapshot", Detail: "head changed during snapshot fetch"}
	}

	pool, err := decodePool(raw)
	if err != nil {
		return &model.StartupError{Code: model.ReasonRPCSchemaError, Stage: "snapshot", Detail: "failed to decode txpool_content", Err: err}
	}

	baseFee, gasLimit, err := parseHeader(header)
	if err != nil {
		return &model.StartupError{Code: model.ReasonRPCSchemaError, Stage: "snapshot", Detail: "invalid block header", Err: err}
	}

	pending, pendingDecisions, err := normalizePool(pool.Pending, baseFee, gasLimit, "pending")
	if err != nil {
		return &model.StartupError{Code: model.ReasonRPCSchemaError, Stage: "snapshot", Detail: "failed to normalize pending pool", Err: err}
	}

	queued := map[string][]model.Transaction{}
	queuedDecisions := []model.TxDecision{}
	if s.cfg.IncludeQueued {
		queued, queuedDecisions, err = normalizePool(pool.Queued, baseFee, gasLimit, "queued")
		if err != nil {
			return &model.StartupError{Code: model.ReasonRPCSchemaError, Stage: "snapshot", Detail: "failed to normalize queued pool", Err: err}
		}
	}

	freshUntil := time.Now().UTC().Add(s.cfg.MaxSnapshotAge)
	snapshot := &model.Snapshot{
		SchemaVersion:   1,
		SnapshotID:      "",
		CapturedAt:      time.Now().UTC(),
		PolicyVersion:   s.cfg.PolicyVersion,
		BinaryVersion:   BinaryVersion,
		ChainID:         chainID.String(),
		BaseFee:         bigString(baseFee),
		GasLimit:        gasLimit,
		MempoolDigest:   digestBytes(raw),
		FreshUntil:      freshUntil,
		RefreshMS:       time.Since(start).Milliseconds(),
		PendingBySender: pending,
		QueuedBySender:  queued,
		SourceLabel:     rpcx.EndpointLabel(s.cfg.RPCURL),
		HeadBefore:      headBefore.String(),
		HeadAfter:       headAfter.String(),
		HeadDrift:       headDrift,
	}
	snapshot.SnapshotID = digestCanonical(snapshotFingerprintFrom(snapshot))

	decisions := make([]model.TxDecision, 0, len(pendingDecisions)+len(queuedDecisions))
	decisions = append(decisions, pendingDecisions...)
	decisions = append(decisions, queuedDecisions...)
	s.recordSnapshot(snapshot, decisions, "")

	if !s.cfg.NoWrite {
		if err := persistSnapshot(s.cfg, snapshot); err != nil {
			return err
		}
	}
	return nil
}
