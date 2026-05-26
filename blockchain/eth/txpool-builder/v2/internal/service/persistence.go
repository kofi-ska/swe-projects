package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"txpool-builder/v2/internal/model"
)

// persistSnapshot keeps the retained epoch on disk so replay can work later.
func persistSnapshot(cfg model.Config, snapshot *model.Snapshot) error {
	if snapshot == nil {
		return nil
	}
	root := filepath.Join(cfg.OutputDir, "snapshots")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	path := filepath.Join(root, snapshot.SnapshotID+".json")
	if err := writeJSONAtomic(path, snapshot, cfg.MaxSnapshotBytes); err != nil {
		return err
	}
	return pruneDir(root, cfg.MaxRetainedSnap)
}

// persistBuildArtifacts writes the candidate and trace together so they stay aligned.
func persistBuildArtifacts(cfg model.Config, candidate model.Candidate, trace model.Trace, snapshot *model.Snapshot) (string, string, error) {
	if snapshot == nil {
		return "", "", fmt.Errorf("snapshot is nil")
	}
	root := filepath.Join(cfg.OutputDir, "results", candidate.CandidateID)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", "", err
	}
	candidatePath := filepath.Join(root, "candidate.json")
	tracePath := filepath.Join(root, "trace.json")
	if err := writeJSONAtomic(candidatePath, candidate, cfg.MaxArtifactBytes); err != nil {
		return "", "", err
	}
	if err := writeJSONAtomic(tracePath, trace, cfg.MaxTraceBytes); err != nil {
		return "", "", err
	}
	_ = pruneDir(filepath.Join(cfg.OutputDir, "results"), cfg.MaxRetainedJobs)
	return candidatePath, tracePath, nil
}

// writeJSONAtomic avoids partial artifact writes by committing only once the file is complete.
func writeJSONAtomic(path string, value any, maxBytes int64) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if maxBytes > 0 && int64(len(b)) > maxBytes {
		return fmt.Errorf("artifact exceeds max bytes: %d > %d", len(b), maxBytes)
	}
	tmp := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// pruneDir keeps history bounded so disk usage does not grow with request volume.
func pruneDir(root string, maxRetained int) error {
	if maxRetained <= 0 {
		return nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	type item struct {
		path string
		mod  time.Time
	}
	items := make([]item, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		items = append(items, item{path: filepath.Join(root, entry.Name()), mod: info.ModTime()})
	}
	if len(items) <= maxRetained {
		return nil
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mod.Before(items[j].mod) })
	for i := 0; i < len(items)-maxRetained; i++ {
		_ = os.RemoveAll(items[i].path)
	}
	return nil
}
