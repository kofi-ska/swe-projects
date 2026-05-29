package run

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"txpool-builder/v1/internal/model"
)

// buildCandidate turns one snapshot into one candidate and one trace.
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

// greedySelect keeps the solver deterministic and bounded instead of chasing optimality.
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
