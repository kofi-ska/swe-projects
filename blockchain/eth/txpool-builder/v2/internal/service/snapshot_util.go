package service

import (
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

// parseHeader extracts just the fields needed for fee math and gas bounds.
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

// parseBigIntString keeps numeric parsing strict without float conversions.
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

// parseUintString keeps count and gas parsing unsigned and deterministic.
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

// inputLen lets traces record payload size without storing the whole blob.
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

// firstNonEmpty preserves the first usable field from upstream payloads.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// bigString keeps JSON output stable for optional big integer fields.
func bigString(v *big.Int) string {
	if v == nil {
		return ""
	}
	return v.String()
}

// compareTx provides a single deterministic ordering for replacements and selection.
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

// rejectDecision standardizes the rejection record so trace summaries stay stable.
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

// bigToString keeps nil big.Int values from leaking into JSON or comparisons.
func bigToString(v *big.Int) string {
	if v == nil {
		return ""
	}
	return v.String()
}

// minBig keeps fee math in integer space when capping effective tips.
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

// snapshotFingerprintFrom narrows the hash input to the fields that define the epoch.
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

// digestBytes fingerprints the raw payload without retaining the whole buffer.
func digestBytes(raw json.RawMessage) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// digestCanonical hashes normalized JSON so replay identity stays reproducible.
func digestCanonical(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// hexNumber preserves the RPC hex format expected by block header lookups.
func hexNumber(n *big.Int) string {
	if n == nil {
		return "0x0"
	}
	return fmt.Sprintf("0x%x", n)
}
