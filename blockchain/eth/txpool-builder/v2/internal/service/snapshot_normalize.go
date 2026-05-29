package service

import (
	"encoding/json"
	"math/big"
	"sort"

	"txpool-builder/v2/internal/model"
)

// normalizePool makes sender chains deterministic before selection sees them.
func normalizePool(pool map[string]map[string]json.RawMessage, baseFee *big.Int, blockGasLimit uint64, stage string) (map[string][]model.Transaction, []model.TxDecision, error) {
	senders := sortedPoolSenders(pool)
	out := make(map[string][]model.Transaction, len(senders))
	decisions := make([]model.TxDecision, 0, len(senders))

	for _, sender := range senders {
		nonces := pool[sender]
		txByNonce := map[uint64][]model.Transaction{}
		for _, nonceKey := range sortedNonceKeys(nonces) {
			tx, decision, err := decodeTx(sender, nonceKey, nonces[nonceKey], baseFee, blockGasLimit, stage)
			if err != nil {
				return nil, nil, err
			}
			if decision != nil {
				decisions = append(decisions, *decision)
				continue
			}
			txByNonce[tx.Nonce] = append(txByNonce[tx.Nonce], tx)
		}
		chain, chainDecisions := normalizeSenderChain(txByNonce, blockGasLimit, stage)
		out[sender] = chain
		decisions = append(decisions, chainDecisions...)
	}
	return out, decisions, nil
}

// normalizeSenderChain removes dominated or invalid suffixes before global ranking.
func normalizeSenderChain(groups map[uint64][]model.Transaction, blockGasLimit uint64, stage string) ([]model.Transaction, []model.TxDecision) {
	nonces := make([]uint64, 0, len(groups))
	for nonce := range groups {
		nonces = append(nonces, nonce)
	}
	sort.Slice(nonces, func(i, j int) bool { return nonces[i] < nonces[j] })

	selected := make([]model.Transaction, 0, len(nonces))
	decisions := make([]model.TxDecision, 0)
	var prev uint64
	var havePrev bool
	for _, nonce := range nonces {
		list := groups[nonce]
		sort.SliceStable(list, func(i, j int) bool {
			return compareTx(list[i], list[j]) < 0
		})
		best := list[0]
		for _, tx := range list[1:] {
			decisions = append(decisions, rejectDecision(tx, model.ReasonReplacementConflict, "same sender and nonce; lower score", stage))
		}
		if havePrev && nonce > prev+1 {
			decisions = append(decisions, rejectDecision(best, model.ReasonNonceGap, "nonce gap in sender chain", stage))
			break
		}
		if blockGasLimit > 0 && best.GasLimit > blockGasLimit {
			decisions = append(decisions, rejectDecision(best, model.ReasonExceedsBlockGas, "tx gas exceeds block gas limit", stage))
			break
		}
		selected = append(selected, best)
		prev = nonce
		havePrev = true
	}
	return selected, decisions
}
