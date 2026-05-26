package run

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"txpool-builder/v1/internal/config"
	"txpool-builder/v1/internal/model"
)

// deterministicID keeps request and artifact identity reproducible from inputs.
func deterministicID(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// DeterministicRunID ties the process identity to config and binary version.
func DeterministicRunID(cfg model.Config) string {
	return deterministicID("run", config.Digest(cfg), BinaryVersion)
}

// digestBytes fingerprints raw payloads without retaining the entire buffer.
func digestBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// digestCanonical hashes normalized JSON so replay identity stays stable.
func digestCanonical(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// hashesOf is the stable projection used in candidate and trace identity.
func hashesOf(txs []model.Transaction) []string {
	out := make([]string, 0, len(txs))
	for _, tx := range txs {
		out = append(out, tx.Hash)
	}
	return out
}

// bigToString keeps optional big integers stable in JSON output.
func bigToString(v *big.Int) string {
	if v == nil {
		return ""
	}
	return v.String()
}

// parseString keeps raw JSON parsing strict and allocation-light.
func parseString(m map[string]json.RawMessage, key string) string {
	raw, ok := m[key]
	if !ok || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}

// parseNullableString preserves nullable fields without extra sentinel values.
func parseNullableString(m map[string]json.RawMessage, key string) *string {
	s := parseString(m, key)
	if s == "" {
		return nil
	}
	return &s
}

// parseUintString keeps count and gas parsing unsigned and deterministic.
func parseUintString(s string) (uint64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		n, err := strconv.ParseUint(s[2:], 16, 64)
		return n, err == nil
	}
	n, err := strconv.ParseUint(s, 10, 64)
	return n, err == nil
}

// parseUint handles string and numeric encodings without floating-point conversion.
func parseUint(m map[string]json.RawMessage, key string) (uint64, bool) {
	raw, ok := m[key]
	if !ok || string(raw) == "null" {
		return 0, false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return parseUintString(s)
	}
	var num json.Number
	if err := json.Unmarshal(raw, &num); err == nil {
		v, err := strconv.ParseUint(num.String(), 10, 64)
		if err == nil {
			return v, true
		}
	}
	return 0, false
}

// parseBig keeps large numeric fields exact.
func parseBig(m map[string]json.RawMessage, key string) (*big.Int, bool) {
	raw, ok := m[key]
	if !ok || string(raw) == "null" {
		return nil, false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		n := parseHexBig(s)
		if n == nil {
			return nil, false
		}
		return n, true
	}
	return nil, false
}

// parseHexBig accepts the hex and decimal forms the RPC layer emits.
func parseHexBig(s string) *big.Int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		n, ok := new(big.Int).SetString(s[2:], 16)
		if ok {
			return n
		}
		return nil
	}
	n, ok := new(big.Int).SetString(s, 10)
	if ok {
		return n
	}
	return nil
}

// parseHexUint64 keeps block header parsing aligned with the RPC wire format.
func parseHexUint64(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return strconv.ParseUint(s[2:], 16, 64)
	}
	return strconv.ParseUint(s, 10, 64)
}

// isValidHexAddress rejects malformed addresses before they enter the model.
func isValidHexAddress(s string) bool {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "0x") || len(s) != 42 {
		return false
	}
	for _, r := range s[2:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// hasBlobFields lets the decoder reject unsupported blob txs early.
func hasBlobFields(obj map[string]json.RawMessage) bool {
	_, ok := obj["blobVersionedHashes"]
	return ok
}

// countPoolEntries gives the trace a cheap pool cardinality metric.
func countPoolEntries(pool map[string]map[string]json.RawMessage) int {
	total := 0
	for _, nonces := range pool {
		total += len(nonces)
	}
	return total
}

// mergeReasonSummary keeps trace summaries additive and deterministic.
func mergeReasonSummary(base map[string]int, buckets decisionBuckets) map[string]int {
	out := map[string]int{}
	for k, v := range base {
		out[k] += v
	}
	inc := func(decisions []model.TxDecision) {
		for _, d := range decisions {
			key := string(d.PrimaryReason)
			if key == "" {
				key = "unknown"
			}
			out[key]++
		}
	}
	inc(buckets.DecodeFailures)
	inc(buckets.ValidationFailures)
	inc(buckets.PolicyRejections)
	inc(buckets.CapacityExclusions)
	return out
}

// mergeReasonMaps is a small compatibility wrapper around summary merging.
func mergeReasonMaps(a map[string]int, b decisionBuckets) map[string]int {
	return mergeReasonSummary(a, b)
}

// mergeSelectionReasonSummary folds selection outcomes into the total reason summary.
func mergeSelectionReasonSummary(a map[string]int, b selectionResult) map[string]int {
	out := map[string]int{}
	for k, v := range a {
		out[k] += v
	}
	for k, v := range b.ReasonSummary {
		out[k] += v
	}
	return out
}

// hexNumber preserves the RPC hex format expected by block header lookups.
func hexNumber(n *big.Int) string {
	if n == nil {
		return "0x0"
	}
	return fmt.Sprintf("0x%x", n)
}
