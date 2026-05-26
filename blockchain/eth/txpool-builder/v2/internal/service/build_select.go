package service

import (
	"container/heap"
	"sort"

	"txpool-builder/v2/internal/model"
)

// selectTransactions keeps packing simple so runtime stays predictable under load.
func selectTransactions(chains map[string][]model.Transaction, maxGas uint64, maxTx int) ([]model.Transaction, []model.TxDecision, []model.TxDecision, []string, string) {
	limit := maxGas
	if limit == 0 {
		limit = ^uint64(0)
	}
	h := &candidateHeap{}
	heap.Init(h)
	senders := make([]string, 0, len(chains))
	for sender := range chains {
		senders = append(senders, sender)
	}
	sort.Strings(senders)
	for _, sender := range senders {
		txs := chains[sender]
		if len(txs) == 0 {
			continue
		}
		heap.Push(h, headCandidate{Sender: sender, Index: 0, Tx: txs[0]})
	}

	selected := make([]model.Transaction, 0, maxTx)
	accepted := make([]model.TxDecision, 0, maxTx)
	rejected := make([]model.TxDecision, 0)
	rankingOrder := make([]string, 0, len(chains))
	remaining := limit
	stopReason := "exhausted"

	for h.Len() > 0 && len(selected) < maxTx {
		cand := heap.Pop(h).(headCandidate)
		rankingOrder = append(rankingOrder, cand.Tx.Hash)
		if cand.Tx.GasLimit > remaining {
			rejected = append(rejected, rejectDecision(cand.Tx, model.ReasonCapacityExcluded, "insufficient remaining gas", "selection"))
			continue
		}
		selected = append(selected, cand.Tx)
		accepted = append(accepted, acceptDecision(cand.Tx, "selection"))
		remaining -= cand.Tx.GasLimit
		nextIndex := cand.Index + 1
		if chain := chains[cand.Sender]; nextIndex < len(chain) {
			heap.Push(h, headCandidate{Sender: cand.Sender, Index: nextIndex, Tx: chain[nextIndex]})
		}
	}

	switch {
	case len(selected) >= maxTx:
		stopReason = "max_transactions"
	case len(selected) == 0 && len(rejected) == 0:
		stopReason = "empty_pool"
	case remaining == 0:
		stopReason = "gas_exhausted"
	}
	return selected, accepted, rejected, rankingOrder, stopReason
}

type headCandidate struct {
	Sender string
	Index  int
	Tx     model.Transaction
}

type candidateHeap []headCandidate

func (h candidateHeap) Len() int { return len(h) }

func (h candidateHeap) Less(i, j int) bool {
	return compareTx(h[i].Tx, h[j].Tx) < 0
}

func (h candidateHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *candidateHeap) Push(x any) {
	*h = append(*h, x.(headCandidate))
}

func (h *candidateHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}
