package service

import (
	"encoding/json"
	"testing"

	"txpool-builder/v2/internal/config"
	"txpool-builder/v2/internal/model"
)

// Snapshot compatibility must survive unknown fields so retained epochs are still readable after upgrades.
func TestSnapshotSchemaCompatibility(t *testing.T) {
	snap := sampleSnapshot()
	b, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	b = append(b[:len(b)-1], []byte(`,"unknown":"field"}`)...)
	var decoded model.Snapshot
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.SnapshotID != snap.SnapshotID || decoded.SchemaVersion != snap.SchemaVersion {
		t.Fatalf("snapshot compatibility broken")
	}
}

// Candidate and trace schemas must keep their identity fields stable across marshal/unmarshal cycles.
func TestCandidateTraceSchemaCompatibility(t *testing.T) {
	candidate := model.Candidate{SchemaVersion: 1, CandidateID: "cand-1", SnapshotID: "snap-1", PolicyVersion: "v2", BinaryVersion: BinaryVersion}
	trace := model.Trace{SchemaVersion: 1, TraceID: "trace-1", SnapshotID: "snap-1", CandidateID: "cand-1", PolicyVersion: "v2", BinaryVersion: BinaryVersion}

	cb, err := json.Marshal(candidate)
	if err != nil {
		t.Fatalf("marshal candidate: %v", err)
	}
	tb, err := json.Marshal(trace)
	if err != nil {
		t.Fatalf("marshal trace: %v", err)
	}

	var decodedCandidate model.Candidate
	var decodedTrace model.Trace
	if err := json.Unmarshal(cb, &decodedCandidate); err != nil {
		t.Fatalf("unmarshal candidate: %v", err)
	}
	if err := json.Unmarshal(tb, &decodedTrace); err != nil {
		t.Fatalf("unmarshal trace: %v", err)
	}
	if decodedCandidate.CandidateID != candidate.CandidateID || decodedTrace.TraceID != trace.TraceID {
		t.Fatalf("compatibility broken")
	}
}

// The config digest must be stable so replay and audit can compare like-for-like runs.
func TestConfigDigestStable(t *testing.T) {
	cfg1 := testConfig()
	cfg2 := testConfig()
	if config.Digest(cfg1) != config.Digest(cfg2) {
		t.Fatalf("digest should be stable")
	}
}
