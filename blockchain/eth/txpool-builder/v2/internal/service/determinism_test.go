package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"txpool-builder/v2/internal/model"
)

// This test proves the same snapshot and policy produce the same candidate fingerprint twice in a row.
func TestBuildCandidateDeterministic(t *testing.T) {
	cfg := testConfig()
	cfg.OutputDir = t.TempDir()
	svc := newTestService(cfg)
	snap := sampleSnapshot()
	req := model.BuildRequest{IdempotencyKey: "idem-1", PriorityClass: "normal", PolicyVersion: "v2"}

	first, trace1, err := svc.buildCandidate(context.Background(), req, snap, nil, time.Unix(1, 0))
	if err != nil {
		t.Fatalf("buildCandidate: %v", err)
	}
	second, trace2, err := svc.buildCandidate(context.Background(), req, snap, nil, time.Unix(1, 0))
	if err != nil {
		t.Fatalf("buildCandidate: %v", err)
	}

	if first.CandidateID != second.CandidateID {
		t.Fatalf("candidate id changed: %s vs %s", first.CandidateID, second.CandidateID)
	}
	if first.TraceID != second.TraceID {
		t.Fatalf("trace id changed: %s vs %s", first.TraceID, second.TraceID)
	}
	if !sameStrings(first.SelectedOrder, second.SelectedOrder) {
		t.Fatalf("selected order changed: %v vs %v", first.SelectedOrder, second.SelectedOrder)
	}
	if !sameMap(first.ReasonSummary, second.ReasonSummary) {
		t.Fatalf("reason summary changed: %v vs %v", first.ReasonSummary, second.ReasonSummary)
	}
	if trace1.CandidateID != trace2.CandidateID || trace1.TraceID != trace2.TraceID {
		t.Fatalf("trace ids changed: %s/%s vs %s/%s", trace1.CandidateID, trace1.TraceID, trace2.CandidateID, trace2.TraceID)
	}
}

// This test protects against map-order drift in the txpool input path.
func TestBuildCandidateStableWithShuffledInput(t *testing.T) {
	cfg := testConfig()
	cfg.OutputDir = t.TempDir()
	svc := newTestService(cfg)
	req := model.BuildRequest{IdempotencyKey: "idem-2", PriorityClass: "normal", PolicyVersion: "v2"}

	left := sampleSnapshot()
	right := sampleSnapshot()
	right.PendingBySender = map[string][]model.Transaction{
		"0xbbb": right.PendingBySender["0xbbb"],
		"0xaaa": right.PendingBySender["0xaaa"],
	}

	a, _, err := svc.buildCandidate(context.Background(), req, left, nil, time.Unix(1, 0))
	if err != nil {
		t.Fatalf("buildCandidate left: %v", err)
	}
	b, _, err := svc.buildCandidate(context.Background(), req, right, nil, time.Unix(1, 0))
	if err != nil {
		t.Fatalf("buildCandidate right: %v", err)
	}

	if a.CandidateID != b.CandidateID {
		t.Fatalf("candidate ids differ: %s vs %s", a.CandidateID, b.CandidateID)
	}
	if !sameStrings(a.SelectedOrder, b.SelectedOrder) {
		t.Fatalf("selected order differs: %v vs %v", a.SelectedOrder, b.SelectedOrder)
	}
}

// This test proves snapshot JSON round-trips without changing stable identity fields.
func TestStableReplayLikeRoundTrip(t *testing.T) {
	snap := sampleSnapshot()
	b, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded model.Snapshot
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.SnapshotID != snap.SnapshotID || decoded.PolicyVersion != snap.PolicyVersion {
		t.Fatalf("round trip changed stable fields")
	}
}

// sameStrings checks that the selected order is byte-stable across runs.
func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sameMap checks that reason counts stay identical when the selection is deterministic.
func sameMap(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
