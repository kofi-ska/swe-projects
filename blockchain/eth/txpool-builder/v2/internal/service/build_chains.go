package service

import (
	"sort"

	"txpool-builder/v2/internal/model"
)

// buildChains keeps per-sender nonce chains isolated before ranking.
func buildChains(snapshot *model.Snapshot, includeQueued bool) map[string][]model.Transaction {
	chains := cloneChains(snapshot.PendingBySender)
	if includeQueued {
		for sender, queued := range snapshot.QueuedBySender {
			chains[sender] = mergeSenderChain(chains[sender], queued)
		}
	}
	return chains
}

// cloneChains avoids mutating retained snapshot state during selection.
func cloneChains(src map[string][]model.Transaction) map[string][]model.Transaction {
	if len(src) == 0 {
		return map[string][]model.Transaction{}
	}
	out := make(map[string][]model.Transaction, len(src))
	for sender, txs := range src {
		if len(txs) == 0 {
			continue
		}
		cloned := make([]model.Transaction, len(txs))
		copy(cloned, txs)
		out[sender] = cloned
	}
	return out
}

// mergeSenderChain resolves queued-versus-pending collisions by nonce.
func mergeSenderChain(existing, queued []model.Transaction) []model.Transaction {
	byNonce := map[uint64]model.Transaction{}
	for _, tx := range existing {
		byNonce[tx.Nonce] = tx
	}
	for _, tx := range queued {
		if prev, ok := byNonce[tx.Nonce]; !ok || compareTx(tx, prev) < 0 {
			byNonce[tx.Nonce] = tx
		}
	}
	nonces := make([]uint64, 0, len(byNonce))
	for nonce := range byNonce {
		nonces = append(nonces, nonce)
	}
	sort.Slice(nonces, func(i, j int) bool { return nonces[i] < nonces[j] })
	out := make([]model.Transaction, 0, len(nonces))
	for _, nonce := range nonces {
		out = append(out, byNonce[nonce])
	}
	return out
}
