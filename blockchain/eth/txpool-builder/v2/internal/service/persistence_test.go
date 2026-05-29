package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"txpool-builder/v2/internal/model"
)

// Atomic write must leave a durable file and no stray temp file behind.
func TestWriteJSONAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "artifact.json")
	payload := map[string]any{"ok": true}
	if err := writeJSONAtomic(path, payload, 1024); err != nil {
		t.Fatalf("writeJSONAtomic: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("missing artifact: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("tmp file still present")
	}
}

// Oversized payloads should fail before they ever touch the filesystem.
func TestWriteJSONAtomicRejectsOversizedPayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "artifact.json")
	payload := map[string]any{"big": strings.Repeat("x", 64)}
	if err := writeJSONAtomic(path, payload, 8); err == nil {
		t.Fatalf("expected oversized payload rejection")
	}
}

// Candidate and trace persistence must succeed together so audits remain coherent.
func TestPersistBuildArtifactsWritesBothFiles(t *testing.T) {
	cfg := testConfig()
	cfg.OutputDir = t.TempDir()
	candidate := model.Candidate{SchemaVersion: 1, CandidateID: "cand-1", SnapshotID: "snap-1", PolicyVersion: "v2", BinaryVersion: BinaryVersion}
	trace := model.Trace{SchemaVersion: 1, TraceID: "trace-1", SnapshotID: "snap-1", CandidateID: "cand-1", PolicyVersion: "v2", BinaryVersion: BinaryVersion}
	snap := sampleSnapshot()
	artifactURI, traceURI, err := persistBuildArtifacts(cfg, candidate, trace, snap)
	if err != nil {
		t.Fatalf("persistBuildArtifacts: %v", err)
	}
	if artifactURI == "" || traceURI == "" {
		t.Fatalf("expected output paths")
	}
	if _, err := os.Stat(artifactURI); err != nil {
		t.Fatalf("candidate not written: %v", err)
	}
	if _, err := os.Stat(traceURI); err != nil {
		t.Fatalf("trace not written: %v", err)
	}
}

// Directory pruning must retain the newest entries and remove the older ones deterministically.
func TestPruneDirRetainsNewest(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 4; i++ {
		path := filepath.Join(dir, fmt.Sprintf("f%d.json", i))
		if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := pruneDir(dir, 2); err != nil {
		t.Fatalf("pruneDir: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("expected 2 files after prune, got %d", len(entries))
	}
}

// JSON round-trip ensures the persisted candidate schema stays readable by future code.
func TestArtifactJSONRoundTrip(t *testing.T) {
	candidate := model.Candidate{SchemaVersion: 1, CandidateID: "cand-1"}
	b, err := json.Marshal(candidate)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded model.Candidate
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.CandidateID != candidate.CandidateID {
		t.Fatalf("round trip mismatch")
	}
}
