package service

import (
	"container/heap"
	"context"
	"math/big"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"txpool-builder/v2/internal/config"
	"txpool-builder/v2/internal/model"
)

func (s *Service) processJob(ctx context.Context, workerID int, env *jobEnvelope) {
	_ = workerID
	startedAt := time.Now().UTC()

	s.mu.Lock()
	job, ok := s.jobs[env.JobID]
	if !ok {
		s.mu.Unlock()
		return
	}
	job.State = model.JobRunning
	job.StartedAt = startedAt
	s.mu.Unlock()

	snapRecord := s.snapshotRecordByID(job.SnapshotID)
	if snapRecord == nil || snapRecord.Snapshot == nil {
		s.failJob(job.JobID, model.ReasonSnapshotTooLarge, "snapshot no longer retained")
		atomic.AddInt64(&s.buildsFailed, 1)
		return
	}

	candidate, trace, err := s.buildCandidate(ctx, env.Request, snapRecord.Snapshot, snapRecord.Decisions, startedAt)
	if err != nil {
		s.failJob(job.JobID, model.ReasonInvariantFailure, err.Error())
		atomic.AddInt64(&s.buildsFailed, 1)
		return
	}

	var artifactURI, traceURI string
	if !s.cfg.NoWrite {
		artifactURI, traceURI, err = persistBuildArtifacts(s.cfg, candidate, trace, snapRecord.Snapshot)
		if err != nil {
			s.failJob(job.JobID, model.ReasonArtifactWriteFailed, err.Error())
			atomic.AddInt64(&s.buildsFailed, 1)
			return
		}
	}

	now := time.Now().UTC()
	result := &model.Result{
		JobID:       job.JobID,
		RequestID:   job.RequestID,
		SnapshotID:  snapRecord.Snapshot.SnapshotID,
		ArtifactURI: artifactURI,
		TraceURI:    traceURI,
		Candidate:   candidate,
		Trace:       trace,
		State:       model.JobCompleted,
		CreatedAt:   job.CreatedAt,
		CompletedAt: now,
	}
	s.recordResult(result)
	s.completeJob(job.JobID)
	atomic.AddInt64(&s.buildsCompleted, 1)
}

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

func buildChains(snapshot *model.Snapshot, includeQueued bool) map[string][]model.Transaction {
	chains := cloneChains(snapshot.PendingBySender)
	if includeQueued {
		for sender, queued := range snapshot.QueuedBySender {
			chains[sender] = mergeSenderChain(chains[sender], queued)
		}
	}
	return chains
}

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
	x := old[n-1]
	*h = old[:n-1]
	return x
}

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

func countReasons(decisions []model.TxDecision) map[string]int {
	out := map[string]int{}
	for _, d := range decisions {
		if d.PrimaryReason == "" {
			continue
		}
		out[string(d.PrimaryReason)]++
	}
	return out
}

func hashesOf(txs []model.Transaction) []string {
	out := make([]string, 0, len(txs))
	for _, tx := range txs {
		out = append(out, tx.Hash)
	}
	return out
}
