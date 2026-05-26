package service

import (
	"context"
	"math/big"
	"strings"
	"time"

	"txpool-builder/v2/internal/config"
	"txpool-builder/v2/internal/model"
)

// buildCandidate turns one snapshot into one auditable candidate.
func (s *Service) buildCandidate(_ context.Context, req model.BuildRequest, snapshot *model.Snapshot, snapshotDecisions []model.TxDecision, startedAt time.Time) (model.Candidate, model.Trace, error) {
	cfgDigest := config.Digest(s.cfg)
	chains := buildChains(snapshot, s.cfg.IncludeQueued)
	selected, accepted, rejected, rankingOrder, stopReason := selectTransactions(chains, s.cfg.MaxGas, s.cfg.MaxTransactions)

	trace := model.Trace{
		SchemaVersion:       1,
		TraceID:             deterministicID("trace", snapshot.SnapshotID, req.IdempotencyKey, cfgDigest),
		SnapshotID:          snapshot.SnapshotID,
		CandidateID:         "",
		PolicyVersion:       s.cfg.PolicyVersion,
		BinaryVersion:       BinaryVersion,
		ConfigDigest:        cfgDigest,
		SourceEndpointLabel: snapshot.SourceLabel,
		ChainID:             snapshot.ChainID,
		ReasonCodeSummary:   map[string]int{},
		Accepted:            make([]model.TxDecision, 0, len(snapshotDecisions)+len(accepted)),
		Rejected:            make([]model.TxDecision, 0, len(snapshotDecisions)+len(rejected)),
		RankingOrder:        rankingOrder,
		SelectionOrder:      hashesOf(selected),
		SelectionStopReason: stopReason,
		CreatedAt:           time.Now().UTC(),
		ReplayMode:          false,
	}
	trace.Rejected = append(trace.Rejected, snapshotDecisions...)
	trace.Accepted = append(trace.Accepted, accepted...)
	trace.Rejected = append(trace.Rejected, rejected...)
	trace.ReasonCodeSummary = countReasons(trace.Rejected)

	totalGas := uint64(0)
	totalRevenue := new(big.Int)
	for _, tx := range selected {
		totalGas += tx.GasLimit
		if tx.Score != nil {
			totalRevenue.Add(totalRevenue, tx.Score)
		}
	}

	selectedHashes := hashesOf(selected)
	candidateID := deterministicID("candidate", snapshot.SnapshotID, req.IdempotencyKey, cfgDigest, strings.Join(selectedHashes, ","))
	trace.CandidateID = candidateID
	trace.TraceID = deterministicID("trace", snapshot.SnapshotID, candidateID, cfgDigest)

	candidate := model.Candidate{
		SchemaVersion:       1,
		CandidateID:         candidateID,
		SnapshotID:          snapshot.SnapshotID,
		PolicyVersion:       s.cfg.PolicyVersion,
		BinaryVersion:       BinaryVersion,
		ConfigDigest:        cfgDigest,
		SourceEndpointLabel: snapshot.SourceLabel,
		ChainID:             snapshot.ChainID,
		SelectedTxs:         selected,
		SelectedOrder:       selectedHashes,
		TxCount:             len(selected),
		TotalGas:            totalGas,
		EstimatedRevenue:    totalRevenue.String(),
		RejectedCount:       len(trace.Rejected),
		ReasonSummary:       trace.ReasonCodeSummary,
		SelectionStopReason: stopReason,
		CreatedAt:           time.Now().UTC(),
		TraceID:             trace.TraceID,
		IsExecutableBlock:   false,
		BuildDurationMS:     time.Since(startedAt).Milliseconds(),
	}
	return candidate, trace, nil
}
