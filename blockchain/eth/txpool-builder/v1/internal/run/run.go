package run

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"txpool-builder/v1/internal/config"
	"txpool-builder/v1/internal/model"
	rpcx "txpool-builder/v1/internal/rpc"
)

var BinaryVersion = "v1-dev"

type Result struct {
	ConfigDigest string
	Candidate    model.BlockCandidate
	Trace        model.DecisionTrace
	Snapshot     canonicalSnapshot
	Replay       bool
	Comparison   *ComparisonResult
}

type ComparisonResult struct {
	Match       bool     `json:"match"`
	Differences []string `json:"differences,omitempty"`
}

type canonicalSnapshot struct {
	SchemaVersion       int                 `json:"schema_version"`
	SnapshotID          string              `json:"snapshot_id"`
	SourceEndpointLabel string              `json:"source_endpoint_label"`
	CapturedAt          time.Time           `json:"captured_at"`
	HeadBefore          string              `json:"head_before"`
	HeadAfter           string              `json:"head_after"`
	HeadDrift           bool                `json:"head_drift"`
	ChainID             string              `json:"chain_id"`
	ClientVersion       string              `json:"client_version,omitempty"`
	FetchDurationMS     int64               `json:"fetch_duration_ms"`
	RawPayloadDigest    string              `json:"raw_payload_digest"`
	RawPendingCount     int                 `json:"raw_pending_count"`
	RawQueuedCount      int                 `json:"raw_queued_count"`
	Pending             []model.Transaction `json:"pending"`
	Queued              []model.Transaction `json:"queued,omitempty"`
	RawSnapshotPath     string              `json:"raw_snapshot_path,omitempty"`
	RawSnapshotSize     int64               `json:"raw_snapshot_size_bytes"`
}

type snapshotFingerprint struct {
	SchemaVersion       int                 `json:"schema_version"`
	SourceEndpointLabel string              `json:"source_endpoint_label"`
	HeadBefore          string              `json:"head_before"`
	HeadAfter           string              `json:"head_after"`
	HeadDrift           bool                `json:"head_drift"`
	ChainID             string              `json:"chain_id"`
	ClientVersion       string              `json:"client_version,omitempty"`
	RawPayloadDigest    string              `json:"raw_payload_digest"`
	RawPendingCount     int                 `json:"raw_pending_count"`
	RawQueuedCount      int                 `json:"raw_queued_count"`
	Pending             []model.Transaction `json:"pending"`
	Queued              []model.Transaction `json:"queued,omitempty"`
	RawSnapshotSize     int64               `json:"raw_snapshot_size_bytes"`
}

type rawPool struct {
	Pending map[string]map[string]json.RawMessage `json:"pending"`
	Queued  map[string]map[string]json.RawMessage `json:"queued"`
}

type senderGroup struct {
	Sender string
	Txs    []model.Transaction
}

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

type decisionBuckets struct {
	DecodeFailures     []model.TxDecision
	ValidationFailures []model.TxDecision
	PolicyRejections   []model.TxDecision
	CapacityExclusions []model.TxDecision
	Accepted           []model.TxDecision
}

func normalizePool(pool map[string]map[string]json.RawMessage, cfg model.Config, chainID *big.Int, header rpcx.BlockHeader, stage string) ([]model.Transaction, decisionBuckets, error) {
	_ = chainID
	baseFee, blockGasLimit, err := parseBlockHeader(header)
	if err != nil {
		return nil, decisionBuckets{}, &model.StartupError{Code: model.ReasonRPCSchemaError, Stage: "snapshot", Detail: "invalid block header", Err: err}
	}
	grouped := make(map[string]map[uint64][]model.Transaction)
	decisions := decisionBuckets{
		DecodeFailures:     []model.TxDecision{},
		ValidationFailures: []model.TxDecision{},
		PolicyRejections:   []model.TxDecision{},
		CapacityExclusions: []model.TxDecision{},
		Accepted:           []model.TxDecision{},
	}

	senders := sortedPoolSenders(pool)
	for _, sender := range senders {
		nonces := pool[sender]
		for _, nonceKey := range sortedNonceKeys(nonces) {
			raw := nonces[nonceKey]
			tx, decision, err := decodeTx(sender, nonceKey, raw, baseFee, blockGasLimit, stage)
			if err != nil {
				if cfg.RejectOnPartialDecode {
					decisions.DecodeFailures = append(decisions.DecodeFailures, model.TxDecision{
						TxHash:        "",
						From:          sender,
						Nonce:         0,
						Accepted:      false,
						PrimaryReason: model.ReasonDecodeError,
						ReasonDetail:  err.Error(),
						Stage:         stage,
					})
					continue
				}
				return nil, decisions, err
			}
			if decision == nil || decision.PrimaryReason == "" {
				grouped[tx.From] = ensureNonceGroup(grouped[tx.From])
				grouped[tx.From][tx.Nonce] = append(grouped[tx.From][tx.Nonce], tx)
				decisions.Accepted = append(decisions.Accepted, model.TxDecision{
					TxHash: tx.Hash, From: tx.From, Nonce: tx.Nonce, Accepted: true, Stage: stage,
					GasLimit: tx.GasLimit, Score: bigToString(tx.Score),
					EffectiveTip:   bigToString(tx.EffectivePriorityFee),
					EffectivePrice: bigToString(tx.EffectiveGasPrice),
				})
			} else if decision.PrimaryReason == model.ReasonPolicyRejected {
				decisions.PolicyRejections = append(decisions.PolicyRejections, *decision)
			} else if decision.PrimaryReason == model.ReasonCapacityExcluded {
				decisions.CapacityExclusions = append(decisions.CapacityExclusions, *decision)
			} else {
				decisions.ValidationFailures = append(decisions.ValidationFailures, *decision)
			}
		}
	}

	eligible := make([]model.Transaction, 0)
	eligibleSenders := make([]string, 0, len(grouped))
	for sender := range grouped {
		eligibleSenders = append(eligibleSenders, sender)
	}
	sort.Strings(eligibleSenders)
	for _, sender := range eligibleSenders {
		groups := grouped[sender]
		senderTxs, senderDecisions := normalizeSenderChains(groups, blockGasLimit)
		eligible = append(eligible, senderTxs...)
		decisions.PolicyRejections = append(decisions.PolicyRejections, senderDecisions.PolicyRejections...)
		decisions.ValidationFailures = append(decisions.ValidationFailures, senderDecisions.ValidationFailures...)
	}
	return eligible, decisions, nil
}

type senderNormalization struct {
	PolicyRejections   []model.TxDecision
	ValidationFailures []model.TxDecision
}

func ensureNonceGroup(m map[uint64][]model.Transaction) map[uint64][]model.Transaction {
	if m == nil {
		return map[uint64][]model.Transaction{}
	}
	return m
}

func sortedPoolSenders(pool map[string]map[string]json.RawMessage) []string {
	senders := make([]string, 0, len(pool))
	for sender := range pool {
		senders = append(senders, sender)
	}
	sort.Strings(senders)
	return senders
}

func sortedNonceKeys(nonces map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(nonces))
	for key := range nonces {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		ni, okI := parseUintString(keys[i])
		nj, okJ := parseUintString(keys[j])
		switch {
		case okI && okJ && ni != nj:
			return ni < nj
		case okI && okJ:
			return keys[i] < keys[j]
		case okI:
			return true
		case okJ:
			return false
		default:
			return keys[i] < keys[j]
		}
	})
	return keys
}

func normalizeSenderChains(groups map[uint64][]model.Transaction, blockGasLimit uint64) ([]model.Transaction, senderNormalization) {
	nonces := make([]uint64, 0, len(groups))
	for n := range groups {
		nonces = append(nonces, n)
	}
	sort.Slice(nonces, func(i, j int) bool { return nonces[i] < nonces[j] })

	out := make([]model.Transaction, 0, len(nonces))
	decisions := senderNormalization{
		PolicyRejections:   []model.TxDecision{},
		ValidationFailures: []model.TxDecision{},
	}
	var prev uint64
	var havePrev bool
	for _, nonce := range nonces {
		list := groups[nonce]
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].Score.Cmp(list[j].Score) != 0 {
				return list[i].Score.Cmp(list[j].Score) > 0
			}
			return list[i].Hash < list[j].Hash
		})
		best := list[0]
		for _, tx := range list[1:] {
			decisions.PolicyRejections = append(decisions.PolicyRejections, model.TxDecision{
				TxHash: tx.Hash, From: tx.From, Nonce: tx.Nonce, Accepted: false, PrimaryReason: model.ReasonReplacementConflict, ReasonDetail: "lower score same nonce", Stage: "policy",
			})
		}
		if havePrev && nonce > prev+1 {
			for _, tx := range list {
				decisions.PolicyRejections = append(decisions.PolicyRejections, model.TxDecision{
					TxHash: tx.Hash, From: tx.From, Nonce: tx.Nonce, Accepted: false, PrimaryReason: model.ReasonNonceGap, ReasonDetail: "nonce gap in sender chain", Stage: "policy",
				})
			}
			break
		}
		if best.GasLimit > blockGasLimit && blockGasLimit > 0 {
			decisions.PolicyRejections = append(decisions.PolicyRejections, model.TxDecision{
				TxHash: best.Hash, From: best.From, Nonce: best.Nonce, Accepted: false, PrimaryReason: model.ReasonExceedsBlockGas, ReasonDetail: "gas limit exceeds block limit", Stage: "policy",
			})
			break
		}
		out = append(out, best)
		prev = nonce
		havePrev = true
	}
	return out, decisions
}

func buildCandidate(ctx context.Context, cfg model.Config, startup model.StartupInfo, snapshot canonicalSnapshot, trace model.DecisionTrace, cfgDigest string, start time.Time) (model.BlockCandidate, model.DecisionTrace, error) {
	_ = ctx
	selected, selectionDecisions, stopReason, rankingOrder, err := greedySelect(snapshot.Pending, cfg)
	if err != nil {
		return model.BlockCandidate{}, trace, err
	}

	trace.SelectionStopReason = stopReason
	trace.RankingOrder = rankingOrder
	trace.SelectionOrder = hashesOf(selected)
	trace.CapacityExclusions = append(trace.CapacityExclusions, selectionDecisions.CapacityExclusions...)
	trace.Accepted = append(trace.Accepted, selectionDecisions.Accepted...)
	trace.ReasonCodeSummary = mergeSelectionReasonSummary(trace.ReasonCodeSummary, selectionDecisions)

	rejectedCount := len(trace.DecodeFailures) + len(trace.ValidationFailures) + len(trace.PolicyRejections) + len(trace.CapacityExclusions)
	totalGas := uint64(0)
	totalRevenue := big.NewInt(0)
	selectedTxs := make([]model.Transaction, len(selected))
	copy(selectedTxs, selected)
	selectedHashes := hashesOf(selectedTxs)
	for _, tx := range selectedTxs {
		totalGas += tx.GasLimit
		if tx.Score != nil {
			totalRevenue.Add(totalRevenue, tx.Score)
		}
	}

	candidateID := deterministicID("candidate", snapshot.SnapshotID, cfg.PolicyVersion, BinaryVersion, cfgDigest, strings.Join(selectedHashes, ","))
	traceID := deterministicID("trace", snapshot.SnapshotID, candidateID, cfg.PolicyVersion, BinaryVersion)
	trace.CandidateID = candidateID
	trace.TraceID = traceID
	trace.ConfigDigest = cfgDigest
	trace.BinaryVersion = BinaryVersion
	trace.PolicyVersion = cfg.PolicyVersion
	trace.SourceEndpointLabel = snapshot.SourceEndpointLabel
	trace.ChainID = snapshot.ChainID
	trace.ReplayMode = trace.ReplayMode || snapshot.RawSnapshotPath != ""
	trace.FinalSummary = fmt.Sprintf("selected=%d rejected=%d stop=%s", len(selectedTxs), rejectedCount, stopReason)
	trace.CreatedAt = time.Now().UTC()

	candidate := model.BlockCandidate{
		SchemaVersion:            1,
		CandidateID:              candidateID,
		SnapshotID:               snapshot.SnapshotID,
		PolicyVersion:            cfg.PolicyVersion,
		BinaryVersion:            BinaryVersion,
		ConfigDigest:             cfgDigest,
		SourceEndpointLabel:      snapshot.SourceEndpointLabel,
		ChainID:                  snapshot.ChainID,
		SelectedTxs:              selectedTxs,
		SelectedTxHashes:         selectedHashes,
		SelectedOrder:            selectedHashes,
		TxCount:                  len(selectedTxs),
		TotalGas:                 totalGas,
		EstimatedPriorityRevenue: totalRevenue.String(),
		RejectedCount:            rejectedCount,
		RejectionSummary:         trace.ReasonCodeSummary,
		SelectionStopReason:      stopReason,
		BuildDurationMS:          time.Since(start).Milliseconds(),
		CreatedAt:                time.Now().UTC(),
		TraceRef:                 traceID,
		IsExecutableBlock:        false,
	}
	return candidate, trace, nil
}

type selectionResult struct {
	Accepted           []model.TxDecision
	CapacityExclusions []model.TxDecision
	ReasonSummary      map[string]int
}

func greedySelect(all []model.Transaction, cfg model.Config) ([]model.Transaction, selectionResult, string, []string, error) {
	bySender := map[string][]model.Transaction{}
	for _, tx := range all {
		bySender[tx.From] = append(bySender[tx.From], tx)
	}
	groups := make([]senderGroup, 0, len(bySender))
	for sender, txs := range bySender {
		sort.SliceStable(txs, func(i, j int) bool {
			if txs[i].Nonce != txs[j].Nonce {
				return txs[i].Nonce < txs[j].Nonce
			}
			if txs[i].Score.Cmp(txs[j].Score) != 0 {
				return txs[i].Score.Cmp(txs[j].Score) > 0
			}
			return txs[i].Hash < txs[j].Hash
		})
		groups = append(groups, senderGroup{Sender: sender, Txs: txs})
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Sender < groups[j].Sender })

	selected := make([]model.Transaction, 0, cfg.MaxTransactions)
	accepted := make([]model.TxDecision, 0)
	capacityExclusions := make([]model.TxDecision, 0)
	reasonSummary := map[string]int{}
	remainingGas := cfg.MaxGas
	stopReason := "no_eligible_candidate"
	heads := make([]int, len(groups))

	for len(selected) < cfg.MaxTransactions {
		candidates := make([]model.Transaction, 0, len(groups))
		for gi := range groups {
			if heads[gi] < len(groups[gi].Txs) {
				candidates = append(candidates, groups[gi].Txs[heads[gi]])
			}
		}
		if len(candidates) == 0 {
			break
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].Score.Cmp(candidates[j].Score) != 0 {
				return candidates[i].Score.Cmp(candidates[j].Score) > 0
			}
			if candidates[i].Nonce != candidates[j].Nonce {
				return candidates[i].Nonce < candidates[j].Nonce
			}
			if candidates[i].From != candidates[j].From {
				return candidates[i].From < candidates[j].From
			}
			return candidates[i].Hash < candidates[j].Hash
		})
		best := candidates[0]
		if best.GasLimit > remainingGas {
			capacityExclusions = append(capacityExclusions, model.TxDecision{
				TxHash: best.Hash, From: best.From, Nonce: best.Nonce, Accepted: false, PrimaryReason: model.ReasonCapacityExcluded, ReasonDetail: "does not fit remaining gas", Stage: "selection", GasLimit: best.GasLimit, Score: bigToString(best.Score), EffectiveTip: bigToString(best.EffectivePriorityFee), EffectivePrice: bigToString(best.EffectiveGasPrice),
			})
			reasonSummary[string(model.ReasonCapacityExcluded)]++
			for gi := range groups {
				if heads[gi] < len(groups[gi].Txs) && groups[gi].Txs[heads[gi]].Hash == best.Hash {
					heads[gi] = len(groups[gi].Txs)
				}
			}
			continue
		}
		selected = append(selected, best)
		accepted = append(accepted, model.TxDecision{
			TxHash: best.Hash, From: best.From, Nonce: best.Nonce, Accepted: true, Stage: "selection", GasLimit: best.GasLimit, Score: bigToString(best.Score), EffectiveTip: bigToString(best.EffectivePriorityFee), EffectivePrice: bigToString(best.EffectiveGasPrice),
		})
		reasonSummary["SELECTED"]++
		remainingGas -= best.GasLimit
		for gi := range groups {
			if heads[gi] < len(groups[gi].Txs) && groups[gi].Txs[heads[gi]].Hash == best.Hash {
				heads[gi]++
				break
			}
		}
		stopReason = "selection_complete"
	}
	if len(selected) == 0 && len(capacityExclusions) > 0 {
		stopReason = "capacity_exhausted"
	}
	rankingOrder := hashesOf(selected)
	return selected, selectionResult{
		Accepted:           accepted,
		CapacityExclusions: capacityExclusions,
		ReasonSummary:      reasonSummary,
	}, stopReason, rankingOrder, nil
}

func decodePool(raw json.RawMessage) (rawPool, error) {
	var top rawPool
	if err := json.Unmarshal(raw, &top); err != nil {
		return rawPool{}, err
	}
	if top.Pending == nil {
		top.Pending = map[string]map[string]json.RawMessage{}
	}
	if top.Queued == nil {
		top.Queued = map[string]map[string]json.RawMessage{}
	}
	return top, nil
}

func loadSnapshot(path string) (canonicalSnapshot, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return canonicalSnapshot{}, err
	}
	var snap canonicalSnapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return canonicalSnapshot{}, err
	}
	if err := validateLoadedSnapshot(snap); err != nil {
		return canonicalSnapshot{}, err
	}
	return snap, nil
}

func validateLoadedSnapshot(snap canonicalSnapshot) error {
	switch {
	case snap.SchemaVersion != 1:
		return fmt.Errorf("unsupported snapshot schema version")
	case snap.SnapshotID == "":
		return fmt.Errorf("missing snapshot id")
	case snap.SourceEndpointLabel == "":
		return fmt.Errorf("missing source endpoint label")
	case snap.ChainID == "":
		return fmt.Errorf("missing chain id")
	case snap.RawPayloadDigest == "":
		return fmt.Errorf("missing raw payload digest")
	case len(snap.Pending) == 0 && len(snap.Queued) == 0:
		return fmt.Errorf("empty replay snapshot")
	}
	return nil
}

func snapshotFingerprintFrom(snapshot canonicalSnapshot) snapshotFingerprint {
	return snapshotFingerprint{
		SchemaVersion:       snapshot.SchemaVersion,
		SourceEndpointLabel: snapshot.SourceEndpointLabel,
		HeadBefore:          snapshot.HeadBefore,
		HeadAfter:           snapshot.HeadAfter,
		HeadDrift:           snapshot.HeadDrift,
		ChainID:             snapshot.ChainID,
		ClientVersion:       snapshot.ClientVersion,
		RawPayloadDigest:    snapshot.RawPayloadDigest,
		RawPendingCount:     snapshot.RawPendingCount,
		RawQueuedCount:      snapshot.RawQueuedCount,
		Pending:             snapshot.Pending,
		Queued:              snapshot.Queued,
		RawSnapshotSize:     snapshot.RawSnapshotSize,
	}
}

func CompareCandidateArtifact(path string, candidate model.BlockCandidate) (*ComparisonResult, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var other model.BlockCandidate
	if err := json.Unmarshal(b, &other); err != nil {
		return nil, err
	}
	diff := compareCandidates(other, candidate)
	return &ComparisonResult{Match: len(diff) == 0, Differences: diff}, nil
}

func compareCandidates(a, b model.BlockCandidate) []string {
	diff := make([]string, 0)
	if a.SnapshotID != b.SnapshotID {
		diff = append(diff, "snapshot_id")
	}
	if a.PolicyVersion != b.PolicyVersion {
		diff = append(diff, "policy_version")
	}
	if a.BinaryVersion != b.BinaryVersion {
		diff = append(diff, "binary_version")
	}
	if a.ConfigDigest != b.ConfigDigest {
		diff = append(diff, "config_digest")
	}
	if a.TxCount != b.TxCount {
		diff = append(diff, "tx_count")
	}
	if a.TotalGas != b.TotalGas {
		diff = append(diff, "total_gas")
	}
	if a.EstimatedPriorityRevenue != b.EstimatedPriorityRevenue {
		diff = append(diff, "estimated_priority_revenue")
	}
	if a.SelectionStopReason != b.SelectionStopReason {
		diff = append(diff, "selection_stop_reason")
	}
	if strings.Join(a.SelectedOrder, ",") != strings.Join(b.SelectedOrder, ",") {
		diff = append(diff, "selected_order")
	}
	if strings.Join(a.SelectedTxHashes, ",") != strings.Join(b.SelectedTxHashes, ",") {
		diff = append(diff, "selected_tx_hashes")
	}
	return diff
}

func traceFromSnapshot(snapshot canonicalSnapshot, cfgDigest string) model.DecisionTrace {
	return model.DecisionTrace{
		SchemaVersion:       1,
		TraceID:             deterministicID("trace", snapshot.SnapshotID, cfgDigest, "replay"),
		SnapshotID:          snapshot.SnapshotID,
		CandidateID:         "",
		PolicyVersion:       "",
		BinaryVersion:       BinaryVersion,
		ConfigDigest:        cfgDigest,
		SourceEndpointLabel: snapshot.SourceEndpointLabel,
		ChainID:             snapshot.ChainID,
		ReasonCodeSummary:   map[string]int{},
		CreatedAt:           time.Now().UTC(),
		ReplayMode:          true,
	}
}

func decodeTx(sender, nonceKey string, raw json.RawMessage, baseFee *big.Int, blockGasLimit uint64, stage string) (model.Transaction, *model.TxDecision, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return model.Transaction{}, &model.TxDecision{TxHash: "", From: sender, Accepted: false, PrimaryReason: model.ReasonDecodeError, ReasonDetail: err.Error(), Stage: stage}, err
	}

	tx := model.Transaction{From: sender, RawMetadata: map[string]any{}, Value: big.NewInt(0)}
	tx.Hash = parseString(obj, "hash")
	if tx.Hash == "" {
		return model.Transaction{}, &model.TxDecision{TxHash: "", From: sender, Accepted: false, PrimaryReason: model.ReasonMissingField, ReasonDetail: "missing hash", Stage: stage}, fmt.Errorf("missing hash")
	}
	from := parseString(obj, "from")
	if from == "" {
		from = sender
	}
	tx.From = from
	if !isValidHexAddress(tx.From) {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInvalidAddress, ReasonDetail: "invalid sender", Stage: stage}, fmt.Errorf("invalid sender")
	}
	to := parseNullableString(obj, "to")
	tx.To = to
	nonce, ok := parseUint(obj, "nonce")
	if !ok {
		nonce, ok = parseUintString(nonceKey)
	}
	if !ok {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonMissingField, ReasonDetail: "missing nonce", Stage: stage}, fmt.Errorf("missing nonce")
	}
	tx.Nonce = nonce
	txType, ok := parseUint(obj, "type")
	if !ok {
		txType = 0
	}
	tx.TxType = uint8(txType)
	if tx.TxType > 2 || hasBlobFields(obj) {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonUnsupportedTxType, ReasonDetail: "unsupported tx type", Stage: stage}, fmt.Errorf("unsupported tx type")
	}
	tx.GasLimit, ok = parseUint(obj, "gas")
	if !ok || tx.GasLimit == 0 {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInvalidGasLimit, ReasonDetail: "invalid gas limit", Stage: stage}, fmt.Errorf("invalid gas limit")
	}
	if blockGasLimit > 0 && tx.GasLimit > blockGasLimit {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonExceedsBlockGas, ReasonDetail: "gas limit exceeds block limit", Stage: stage}, fmt.Errorf("gas limit exceeds block")
	}
	if tx.TxType == 0 || tx.TxType == 1 {
		tx.GasPrice, _ = parseBig(obj, "gasPrice")
		if tx.GasPrice == nil {
			return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonMissingField, ReasonDetail: "missing gasPrice", Stage: stage}, fmt.Errorf("missing gasPrice")
		}
	}
	if tx.TxType == 2 {
		tx.MaxFeePerGas, _ = parseBig(obj, "maxFeePerGas")
		tx.MaxPriorityFeePerGas, _ = parseBig(obj, "maxPriorityFeePerGas")
		if tx.MaxFeePerGas == nil || tx.MaxPriorityFeePerGas == nil {
			return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonMissingField, ReasonDetail: "missing dynamic fee fields", Stage: stage}, fmt.Errorf("missing dynamic fee fields")
		}
		if baseFee == nil {
			return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInvalidFeeModel, ReasonDetail: "missing base fee for dynamic fee tx", Stage: stage}, fmt.Errorf("missing base fee")
		}
		if tx.MaxFeePerGas.Cmp(baseFee) < 0 {
			return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInsufficientEffectiveFee, ReasonDetail: "maxFeePerGas below baseFee", Stage: stage}, fmt.Errorf("maxFeePerGas below baseFee")
		}
	}
	tx.Value, _ = parseBig(obj, "value")
	if tx.Value == nil {
		tx.Value = big.NewInt(0)
	}
	input := parseString(obj, "input")
	if input == "" {
		input = parseString(obj, "data")
	}
	if !strings.HasPrefix(input, "0x") {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInvalidHex, ReasonDetail: "invalid input hex", Stage: stage}, fmt.Errorf("invalid input hex")
	}
	dataBytes, err := hex.DecodeString(strings.TrimPrefix(input, "0x"))
	if err != nil {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInvalidHex, ReasonDetail: err.Error(), Stage: stage}, err
	}
	if len(dataBytes) > 0 {
		// no-op; data length is stored below
	}
	tx.InputLen = len(dataBytes)
	tx.AccessList = countAccessList(obj["accessList"])
	tx.IntrinsicGas, err = computeIntrinsicGas(tx, dataBytes)
	if err != nil {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInvalidGasLimit, ReasonDetail: err.Error(), Stage: stage}, err
	}
	if tx.GasLimit < tx.IntrinsicGas {
		return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInvalidGasLimit, ReasonDetail: "gas below intrinsic gas", Stage: stage}, fmt.Errorf("gas below intrinsic gas")
	}
	if tx.TxType == 0 || tx.TxType == 1 {
		tx.EffectiveGasPrice = new(big.Int).Set(tx.GasPrice)
		tx.EffectivePriorityFee = new(big.Int).Set(tx.GasPrice)
		if baseFee != nil {
			tx.EffectivePriorityFee = new(big.Int).Sub(tx.GasPrice, baseFee)
			if tx.EffectivePriorityFee.Sign() < 0 {
				return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInsufficientEffectiveFee, ReasonDetail: "gas price below base fee", Stage: stage}, fmt.Errorf("gas price below base fee")
			}
		}
	}
	if tx.TxType == 2 {
		tip := new(big.Int).Sub(tx.MaxFeePerGas, baseFee)
		if tip.Sign() < 0 {
			return model.Transaction{}, &model.TxDecision{TxHash: tx.Hash, From: tx.From, Accepted: false, PrimaryReason: model.ReasonInsufficientEffectiveFee, ReasonDetail: "maxFeePerGas below baseFee", Stage: stage}, fmt.Errorf("maxFeePerGas below baseFee")
		}
		if tip.Cmp(tx.MaxPriorityFeePerGas) > 0 {
			tip = new(big.Int).Set(tx.MaxPriorityFeePerGas)
		}
		tx.EffectivePriorityFee = tip
		tx.EffectiveGasPrice = new(big.Int).Add(baseFee, tip)
	}
	tx.Score = new(big.Int).Mul(tx.EffectivePriorityFee, new(big.Int).SetUint64(tx.GasLimit))
	return tx, nil, nil
}

func parseBlockHeader(header rpcx.BlockHeader) (*big.Int, uint64, error) {
	var baseFee *big.Int
	if header.BaseFeePerGas != "" && header.BaseFeePerGas != "null" {
		baseFee = parseHexBig(header.BaseFeePerGas)
		if baseFee == nil {
			return nil, 0, fmt.Errorf("invalid base fee")
		}
	}
	gasLimit := uint64(0)
	if header.GasLimit != "" && header.GasLimit != "null" {
		v, err := parseHexUint64(header.GasLimit)
		if err != nil {
			return nil, 0, err
		}
		gasLimit = v
	}
	return baseFee, gasLimit, nil
}

func computeIntrinsicGas(tx model.Transaction, data []byte) (uint64, error) {
	const (
		txGas                = 21000
		txCreationGas        = 32000
		txZeroByteGas        = 4
		txNonZeroByteGas     = 16
		accessListAddressGas = 2400
		accessListStorageGas = 1900
	)
	gas := uint64(txGas)
	if tx.To == nil {
		gas += txCreationGas
	}
	for _, b := range data {
		if b == 0 {
			gas += txZeroByteGas
		} else {
			gas += txNonZeroByteGas
		}
	}
	if tx.AccessList > 0 {
		gas += accessListAddressGas
		gas += accessListStorageGas
	}
	return gas, nil
}

func countAccessList(raw json.RawMessage) int {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var arr []any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return 0
	}
	return len(arr)
}

func writeSnapshot(path string, snapshot canonicalSnapshot, maxBytes int64) error {
	return writeAtomic(path, snapshot, maxBytes)
}

func writeAtomic(path string, value any, maxBytes int64) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return fmt.Errorf("artifact exceeds max bytes")
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".txpool-builder-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func ensureWritable(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".writetest-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(name)
	return nil
}

func deterministicID(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func DeterministicRunID(cfg model.Config) string {
	return deterministicID("run", config.Digest(cfg), BinaryVersion)
}

func digestBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func digestCanonical(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hashesOf(txs []model.Transaction) []string {
	out := make([]string, 0, len(txs))
	for _, tx := range txs {
		out = append(out, tx.Hash)
	}
	return out
}

func bigToString(v *big.Int) string {
	if v == nil {
		return ""
	}
	return v.String()
}

func parseString(m map[string]json.RawMessage, key string) string {
	raw, ok := m[key]
	if !ok || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

func parseNullableString(m map[string]json.RawMessage, key string) *string {
	s := parseString(m, key)
	if s == "" {
		return nil
	}
	return &s
}

func parseUintString(s string) (uint64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		n, err := strconv.ParseUint(s[2:], 16, 64)
		return n, err == nil
	}
	n, err := strconv.ParseUint(s, 10, 64)
	return n, err == nil
}

func parseUint(m map[string]json.RawMessage, key string) (uint64, bool) {
	raw, ok := m[key]
	if !ok || string(raw) == "null" {
		return 0, false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return parseUintString(s)
	}
	var num json.Number
	if err := json.Unmarshal(raw, &num); err == nil {
		v, err := strconv.ParseUint(num.String(), 10, 64)
		if err == nil {
			return v, true
		}
	}
	return 0, false
}

func parseBig(m map[string]json.RawMessage, key string) (*big.Int, bool) {
	raw, ok := m[key]
	if !ok || string(raw) == "null" {
		return nil, false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		n := parseHexBig(s)
		if n == nil {
			return nil, false
		}
		return n, true
	}
	return nil, false
}

func parseHexBig(s string) *big.Int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		n, ok := new(big.Int).SetString(s[2:], 16)
		if ok {
			return n
		}
		return nil
	}
	n, ok := new(big.Int).SetString(s, 10)
	if ok {
		return n
	}
	return nil
}

func parseHexUint64(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strconv.ParseUint(s[2:], 16, 64)
	}
	return strconv.ParseUint(s, 10, 64)
}

func isValidHexAddress(s string) bool {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "0x") || len(s) != 42 {
		return false
	}
	for _, r := range s[2:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func hasBlobFields(obj map[string]json.RawMessage) bool {
	_, blobHashes := obj["blobVersionedHashes"]
	_, blobFee := obj["maxFeePerBlobGas"]
	return blobHashes || blobFee
}

func countPoolEntries(pool map[string]map[string]json.RawMessage) int {
	count := 0
	for _, nonces := range pool {
		count += len(nonces)
	}
	return count
}

func mergeReasonSummary(base map[string]int, buckets decisionBuckets) map[string]int {
	if base == nil {
		base = map[string]int{}
	}
	for _, d := range buckets.DecodeFailures {
		base[string(d.PrimaryReason)]++
	}
	for _, d := range buckets.ValidationFailures {
		base[string(d.PrimaryReason)]++
	}
	for _, d := range buckets.PolicyRejections {
		base[string(d.PrimaryReason)]++
	}
	for _, d := range buckets.CapacityExclusions {
		base[string(d.PrimaryReason)]++
	}
	for _, d := range buckets.Accepted {
		if d.Accepted {
			base["SELECTED"]++
		}
	}
	return base
}

func mergeReasonMaps(a map[string]int, b decisionBuckets) map[string]int {
	return mergeReasonSummary(a, b)
}

func mergeSelectionReasonSummary(a map[string]int, b selectionResult) map[string]int {
	if a == nil {
		a = map[string]int{}
	}
	for _, d := range b.Accepted {
		if d.Accepted {
			a["SELECTED"]++
		}
	}
	for _, d := range b.CapacityExclusions {
		a[string(d.PrimaryReason)]++
	}
	return a
}

func hexNumber(n *big.Int) string {
	if n == nil {
		return "0x0"
	}
	return "0x" + strings.TrimLeft(strings.ToLower(n.Text(16)), "0")
}
