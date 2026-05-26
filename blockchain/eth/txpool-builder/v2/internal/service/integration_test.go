package service

import (
	"context"
	"testing"

	"txpool-builder/v2/internal/model"
)

// The end-to-end flow must move from admission to durable completion without dropping job state.
func TestAdmissionToCompletionFlow(t *testing.T) {
	cfg := testConfig()
	cfg.OutputDir = t.TempDir()
	cfg.NoWrite = true
	svc := newTestService(cfg)
	snap := sampleSnapshot()
	svc.recordSnapshot(snap, nil, "")

	resp, code, err := svc.Submit(context.Background(), model.BuildRequest{IdempotencyKey: "flow-1", PriorityClass: "normal", PolicyVersion: "v2"})
	if err != nil || code != 200 && code != 202 {
		t.Fatalf("submit failed: code=%d err=%v", code, err)
	}

	svc.processJob(context.Background(), 0, &jobEnvelope{Request: model.BuildRequest{IdempotencyKey: "flow-1", PriorityClass: "normal", PolicyVersion: "v2"}, JobID: resp.JobID})

	svc.mu.RLock()
	defer svc.mu.RUnlock()
	job := svc.jobs[resp.JobID]
	if job == nil || job.State != model.JobCompleted {
		t.Fatalf("expected completed job, got %+v", job)
	}
	if _, ok := svc.results[resp.JobID]; !ok {
		t.Fatalf("missing result record")
	}
}
