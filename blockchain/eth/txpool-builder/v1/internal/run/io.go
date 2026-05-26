package run

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"txpool-builder/v1/internal/model"
)

// loadSnapshot keeps replay reads separate from live capture.
func loadSnapshot(path string) (canonicalSnapshot, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return canonicalSnapshot{}, err
	}
	var snap canonicalSnapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return canonicalSnapshot{}, err
	}
	if err := validateLoadedSnapshot(snap); err != nil {
		return canonicalSnapshot{}, err
	}
	return snap, nil
}

// CompareCandidateArtifact gives replay a cheap equality check against a saved candidate.
func CompareCandidateArtifact(path string, candidate model.BlockCandidate) (*ComparisonResult, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var other model.BlockCandidate
	if err := json.Unmarshal(b, &other); err != nil {
		return nil, err
	}
	diff := compareCandidates(other, candidate)
	return &ComparisonResult{Match: len(diff) == 0, Differences: diff}, nil
}

// compareCandidates keeps replay diffs small and deterministic.
func compareCandidates(a, b model.BlockCandidate) []string {
	diff := make([]string, 0)
	if a.SnapshotID != b.SnapshotID {
		diff = append(diff, "snapshot_id")
	}
	if a.PolicyVersion != b.PolicyVersion {
		diff = append(diff, "policy_version")
	}
	if a.BinaryVersion != b.BinaryVersion {
		diff = append(diff, "binary_version")
	}
	if a.ConfigDigest != b.ConfigDigest {
		diff = append(diff, "config_digest")
	}
	if a.TxCount != b.TxCount {
		diff = append(diff, "tx_count")
	}
	if a.TotalGas != b.TotalGas {
		diff = append(diff, "total_gas")
	}
	if a.EstimatedPriorityRevenue != b.EstimatedPriorityRevenue {
		diff = append(diff, "estimated_priority_revenue")
	}
	if a.SelectionStopReason != b.SelectionStopReason {
		diff = append(diff, "selection_stop_reason")
	}
	if strings.Join(a.SelectedOrder, ",") != strings.Join(b.SelectedOrder, ",") {
		diff = append(diff, "selected_order")
	}
	if strings.Join(a.SelectedTxHashes, ",") != strings.Join(b.SelectedTxHashes, ",") {
		diff = append(diff, "selected_tx_hashes")
	}
	return diff
}

// writeSnapshot keeps snapshot persistence as a thin wrapper over atomic writes.
func writeSnapshot(path string, snapshot canonicalSnapshot, maxBytes int64) error {
	return writeAtomic(path, snapshot, maxBytes)
}

// writeAtomic prevents partial files from becoming visible to later reads.
func writeAtomic(path string, value any, maxBytes int64) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return fmt.Errorf("artifact exceeds max bytes")
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".txpool-builder-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// ensureWritable checks output paths up front so startup failures happen early.
func ensureWritable(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".writetest-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_ = tmp.Close()
	_ = os.Remove(name)
	return nil
}
