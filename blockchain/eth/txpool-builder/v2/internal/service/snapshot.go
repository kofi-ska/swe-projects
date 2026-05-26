package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"txpool-builder/v2/internal/model"
	rpcx "txpool-builder/v2/internal/rpc"
)

type rawPool struct {
	Pending map[string]map[string]json.RawMessage `json:"pending"`
	Queued  map[string]map[string]json.RawMessage `json:"queued"`
}

type rawTransaction struct {
	Hash                 string `json:"hash"`
	From                 string `json:"from"`
	To                   string `json:"to"`
	Gas                  string `json:"gas"`
	GasPrice             string `json:"gasPrice"`
	MaxFeePerGas         string `json:"maxFeePerGas"`
	MaxPriorityFeePerGas string `json:"maxPriorityFeePerGas"`
	Value                string `json:"value"`
	Type                 string `json:"type"`
	Input                string `json:"input"`
}

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

func decodePool(raw json.RawMessage) (rawPool, error) {
	var pool rawPool
	if err := json.Unmarshal(raw, &pool); err != nil {
		return rawPool{}, err
	}
	if pool.Pending == nil {
		pool.Pending = map[string]map[string]json.RawMessage{}
	}
	if pool.Queued == nil {
		pool.Queued = map[string]map[string]json.RawMessage{}
	}
	return pool, nil
}

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

func decodeTx(sender, nonceKey string, raw json.RawMessage, baseFee *big.Int, blockGasLimit uint64, stage string) (model.Transaction, *model.TxDecision, error) {
	var txraw rawTransaction
	if err := json.Unmarshal(raw, &txraw); err != nil {
		return model.Transaction{}, nil, err
	}

	nonce, ok := parseUintString(nonceKey)
	if !ok {
		decision := rejectDecision(model.Transaction{From: sender}, model.ReasonInvalidNonce, "invalid nonce key", stage)
		return model.Transaction{}, &decision, nil
	}
	if txraw.Hash == "" {
		decision := rejectDecision(model.Transaction{From: sender, Nonce: nonce}, model.ReasonMissingField, "missing tx hash", stage)
		return model.Transaction{}, &decision, nil
	}
	gasLimit, ok := parseUintString(txraw.Gas)
	if !ok || gasLimit == 0 {
		decision := rejectDecision(model.Transaction{Hash: txraw.Hash, From: sender, Nonce: nonce}, model.ReasonInvalidGas, "invalid gas", stage)
		return model.Transaction{}, &decision, nil
	}
	if blockGasLimit > 0 && gasLimit > blockGasLimit {
		decision := rejectDecision(model.Transaction{Hash: txraw.Hash, From: sender, Nonce: nonce, GasLimit: gasLimit}, model.ReasonExceedsBlockGas, "tx gas exceeds block gas limit", stage)
		return model.Transaction{}, &decision, nil
	}

	txType, _ := parseUintString(txraw.Type)
	tx := model.Transaction{
		Hash:     txraw.Hash,
		From:     firstNonEmpty(txraw.From, sender),
		To:       txraw.To,
		Nonce:    nonce,
		TxType:   uint8(txType),
		GasLimit: gasLimit,
		InputLen: inputLen(txraw.Input),
	}

	value, ok := parseBigIntString(txraw.Value)
	if ok {
		tx.Value = value
	}

	if err := resolveFeeFields(&tx, txraw, baseFee); err != nil {
		decision := rejectDecision(tx, model.ReasonInvalidFeeModel, err.Error(), stage)
		return model.Transaction{}, &decision, nil
	}
	return tx, nil, nil
}

func resolveFeeFields(tx *model.Transaction, txraw rawTransaction, baseFee *big.Int) error {
	switch {
	case txraw.MaxFeePerGas != "" || txraw.MaxPriorityFeePerGas != "":
		if baseFee == nil {
			return fmt.Errorf("dynamic fee tx requires base fee")
		}
		maxFee, ok := parseBigIntString(txraw.MaxFeePerGas)
		if !ok {
			return fmt.Errorf("invalid maxFeePerGas")
		}
		maxTip, ok := parseBigIntString(txraw.MaxPriorityFeePerGas)
		if !ok {
			return fmt.Errorf("invalid maxPriorityFeePerGas")
		}
		if maxFee.Cmp(baseFee) < 0 {
			return fmt.Errorf("maxFeePerGas below base fee")
		}
		priorityCap := new(big.Int).Sub(maxFee, baseFee)
		effectiveTip := minBig(maxTip, priorityCap)
		tx.MaxFeePerGas = maxFee
		tx.MaxPriorityFeePerGas = maxTip
		tx.EffectiveFee = effectiveTip
		tx.Score = new(big.Int).Mul(new(big.Int).Set(effectiveTip), new(big.Int).SetUint64(tx.GasLimit))
		return nil
	case txraw.GasPrice != "":
		gasPrice, ok := parseBigIntString(txraw.GasPrice)
		if !ok {
			return fmt.Errorf("invalid gasPrice")
		}
		tx.GasPrice = gasPrice
		tx.EffectiveFee = gasPrice
		tx.Score = new(big.Int).Mul(new(big.Int).Set(gasPrice), new(big.Int).SetUint64(tx.GasLimit))
		return nil
	default:
		return fmt.Errorf("missing fee fields")
	}
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

func parseHeader(header rpcx.BlockHeader) (*big.Int, uint64, error) {
	baseFee, ok := parseBigIntString(header.BaseFeePerGas)
	if header.BaseFeePerGas != "" && !ok {
		return nil, 0, fmt.Errorf("invalid base fee")
	}
	gasLimit, ok := parseUintString(header.GasLimit)
	if !ok {
		return nil, 0, fmt.Errorf("invalid gas limit")
	}
	return baseFee, gasLimit, nil
}

func parseBigIntString(s string) (*big.Int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		if n, ok := new(big.Int).SetString(s[2:], 16); ok {
			return n, true
		}
		return nil, false
	}
	if n, ok := new(big.Int).SetString(s, 10); ok {
		return n, true
	}
	return nil, false
}

func parseUintString(s string) (uint64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		n, err := strconv.ParseUint(s[2:], 16, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func inputLen(input string) int {
	input = strings.TrimSpace(input)
	if input == "" {
		return 0
	}
	if strings.HasPrefix(input, "0x") || strings.HasPrefix(input, "0X") {
		return len(input[2:]) / 2
	}
	return len(input) / 2
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func bigString(v *big.Int) string {
	if v == nil {
		return ""
	}
	return v.String()
}

func compareTx(a, b model.Transaction) int {
	switch {
	case a.Score != nil && b.Score != nil:
		if cmp := a.Score.Cmp(b.Score); cmp != 0 {
			return -cmp
		}
	case a.Score != nil:
		return -1
	case b.Score != nil:
		return 1
	}
	if a.GasLimit != b.GasLimit {
		if a.GasLimit < b.GasLimit {
			return -1
		}
		return 1
	}
	if a.Hash < b.Hash {
		return -1
	}
	if a.Hash > b.Hash {
		return 1
	}
	return 0
}

func rejectDecision(tx model.Transaction, code model.ReasonCode, detail, stage string) model.TxDecision {
	return model.TxDecision{
		TxHash:        tx.Hash,
		From:          tx.From,
		Nonce:         tx.Nonce,
		Accepted:      false,
		PrimaryReason: code,
		ReasonDetail:  detail,
		Stage:         stage,
		GasLimit:      tx.GasLimit,
		Score:         bigToString(tx.Score),
	}
}

func bigToString(v *big.Int) string {
	if v == nil {
		return ""
	}
	return v.String()
}

func minBig(a, b *big.Int) *big.Int {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	if a.Cmp(b) <= 0 {
		return a
	}
	return b
}

func snapshotFingerprintFrom(snapshot *model.Snapshot) snapshotFingerprint {
	return snapshotFingerprint{
		SchemaVersion:   snapshot.SchemaVersion,
		SourceLabel:     snapshot.SourceLabel,
		HeadBefore:      snapshot.HeadBefore,
		HeadAfter:       snapshot.HeadAfter,
		HeadDrift:       snapshot.HeadDrift,
		ChainID:         snapshot.ChainID,
		BaseFee:         snapshot.BaseFee,
		GasLimit:        snapshot.GasLimit,
		MempoolDigest:   snapshot.MempoolDigest,
		FreshUntil:      snapshot.FreshUntil,
		RefreshMS:       snapshot.RefreshMS,
		PendingBySender: snapshot.PendingBySender,
		QueuedBySender:  snapshot.QueuedBySender,
	}
}

type snapshotFingerprint struct {
	SchemaVersion   int                            `json:"schema_version"`
	SourceLabel     string                         `json:"source_label"`
	HeadBefore      string                         `json:"head_before"`
	HeadAfter       string                         `json:"head_after"`
	HeadDrift       bool                           `json:"head_drift"`
	ChainID         string                         `json:"chain_id"`
	BaseFee         string                         `json:"base_fee,omitempty"`
	GasLimit        uint64                         `json:"gas_limit"`
	MempoolDigest   string                         `json:"mempool_digest"`
	FreshUntil      time.Time                      `json:"fresh_until"`
	RefreshMS       int64                          `json:"refresh_ms"`
	PendingBySender map[string][]model.Transaction `json:"pending_by_sender,omitempty"`
	QueuedBySender  map[string][]model.Transaction `json:"queued_by_sender,omitempty"`
}

func digestBytes(raw json.RawMessage) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func digestCanonical(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hexNumber(n *big.Int) string {
	if n == nil {
		return "0x0"
	}
	return fmt.Sprintf("0x%x", n)
}
