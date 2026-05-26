package run

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"time"

	"txpool-builder/v1/internal/model"
	rpcx "txpool-builder/v1/internal/rpc"
)

// decodePool rejects malformed txpool JSON before normalization touches it.
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

// validateLoadedSnapshot protects replay from partially written or stale snapshots.
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

// snapshotFingerprintFrom keeps replay identity on stable snapshot fields only.
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

// traceFromSnapshot makes replay mode explicit without live RPC side effects.
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

// normalizePool turns raw pool JSON into typed sender chains with explicit reasons.
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

// ensureNonceGroup avoids nil map writes while building sender chains.
func ensureNonceGroup(m map[uint64][]model.Transaction) map[uint64][]model.Transaction {
	if m == nil {
		return map[uint64][]model.Transaction{}
	}
	return m
}

// sortedPoolSenders removes map-order nondeterminism before normalization.
func sortedPoolSenders(pool map[string]map[string]json.RawMessage) []string {
	senders := make([]string, 0, len(pool))
	for sender := range pool {
		senders = append(senders, sender)
	}
	sort.Strings(senders)
	return senders
}

// sortedNonceKeys keeps nonce iteration stable even when upstream encodes strings.
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

// normalizeSenderChains removes replacement conflicts and nonce gaps per sender.
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
