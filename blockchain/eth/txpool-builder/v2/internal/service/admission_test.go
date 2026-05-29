package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"txpool-builder/v2/internal/model"
)

// Missing idempotency keys must fail fast so admission stays predictable and cheap.
func TestSubmitRequiresIdempotencyKey(t *testing.T) {
	svc := newTestService(testConfig())
	svc.setSnapshot(sampleSnapshot())

	_, code, err := svc.Submit(context.Background(), model.BuildRequest{})
	if code != http.StatusBadRequest || err == nil {
		t.Fatalf("expected bad request for missing idempotency key, got code=%d err=%v", code, err)
	}
}

// Duplicate idempotency keys must resolve to the same job so retries do not amplify load.
func TestSubmitDuplicateIdempotencyReturnsSameJob(t *testing.T) {
	svc := newTestService(testConfig())
	svc.setSnapshot(sampleSnapshot())

	first, code, err := svc.Submit(context.Background(), model.BuildRequest{IdempotencyKey: "dup-1"})
	if err != nil || code != http.StatusAccepted {
		t.Fatalf("first submit failed: code=%d err=%v", code, err)
	}
	second, code, err := svc.Submit(context.Background(), model.BuildRequest{IdempotencyKey: "dup-1"})
	if err != nil || code != http.StatusAccepted {
		t.Fatalf("second submit failed: code=%d err=%v", code, err)
	}
	if first.JobID != second.JobID || first.RequestID != second.RequestID {
		t.Fatalf("duplicate idempotency returned different admission response")
	}
}

// Queue saturation should shed immediately instead of blocking or growing unbounded.
func TestSubmitShedsWhenQueueFull(t *testing.T) {
	cfg := testConfig()
	cfg.QueueSize = 1
	svc := newTestService(cfg)
	svc.setSnapshot(sampleSnapshot())
	svc.queue <- &jobEnvelope{Request: model.BuildRequest{IdempotencyKey: "seed", PriorityClass: "normal"}, JobID: "job-seed"}

	_, code, err := svc.Submit(context.Background(), model.BuildRequest{IdempotencyKey: "overload-1"})
	if code != http.StatusTooManyRequests || err == nil {
		t.Fatalf("expected shed response, got code=%d err=%v", code, err)
	}
	if got := len(svc.queue); got != 1 {
		t.Fatalf("queue depth changed unexpectedly: %d", got)
	}
}

// Status must expose stale snapshot and queue saturation as degraded mode signals.
func TestStatusDegradedWhenSnapshotStaleOrQueueFull(t *testing.T) {
	cfg := testConfig()
	svc := newTestService(cfg)
	snap := sampleSnapshot()
	snap.CapturedAt = time.Unix(0, 0)
	svc.setSnapshot(snap)

	st := svc.Status()
	if st.Mode != "degraded" {
		t.Fatalf("expected degraded mode for stale snapshot, got %s", st.Mode)
	}

	svc.queue <- &jobEnvelope{Request: model.BuildRequest{IdempotencyKey: "seed2"}, JobID: "job-seed2"}
	st = svc.Status()
	if st.Mode != "degraded" {
		t.Fatalf("expected degraded mode for queue saturation, got %s", st.Mode)
	}
}
