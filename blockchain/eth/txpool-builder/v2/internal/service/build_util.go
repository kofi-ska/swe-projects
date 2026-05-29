package service

import "txpool-builder/v2/internal/model"

// acceptDecision is separate so traces can distinguish selection from rejection cleanly.
func acceptDecision(tx model.Transaction, stage string) model.TxDecision {
	return model.TxDecision{
		TxHash:   tx.Hash,
		From:     tx.From,
		Nonce:    tx.Nonce,
		Accepted: true,
		Stage:    stage,
		GasLimit: tx.GasLimit,
		Score:    bigToString(tx.Score),
	}
}

// countReasons keeps trace summaries small and deterministic.
func countReasons(decisions []model.TxDecision) map[string]int {
	counts := map[string]int{}
	for _, d := range decisions {
		key := string(d.PrimaryReason)
		if key == "" {
			key = "unknown"
		}
		counts[key]++
	}
	return counts
}

// hashesOf is a stable projection used in candidate and trace identities.
func hashesOf(txs []model.Transaction) []string {
	out := make([]string, 0, len(txs))
	for _, tx := range txs {
		out = append(out, tx.Hash)
	}
	return out
}
