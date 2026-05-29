package relay

import (
	"testing"
	"time"

	"mevrelayv3/internal/config"
	"mevrelayv3/internal/telemetry"
)

func TestControlPolicyAdaptsToPressure(t *testing.T) {
	cfg := config.Config{
		MinNetValue:      0.1,
		MinDeadlineSlack: 500 * time.Millisecond,
		MaxQueueAge:      3 * time.Second,
		RetryBackoff:     500 * time.Millisecond,
		RequestTimeout:   2 * time.Second,
		HistoryLimit:     256,
	}
	p := NewControlPolicy(cfg)
	base := p.Snapshot()

	snap := telemetry.Snapshot{
		RetryDebt:           20,
		BackendLatencyMS:    1500,
		StateLatencyMS:      1000,
		BrokerLatencyMS:     1000,
		WALLatencyMS:        1000,
		CheckpointLatencyMS: 1000,
	}
	report := HealthReport{
		State:            HealthStateDegraded,
		Ready:            false,
		QueueDepth:       960,
		QueueCap:         1024,
		QueueOldestAgeMS: 2500,
		QueueNetValue:    -10,
		RetryPending:     12,
		AuthorityFresh:   true,
	}

	view := p.Adapt(cfg, snap, report)
	if view.Revision <= base.Revision {
		t.Fatalf("revision did not advance: base=%d got=%d", base.Revision, view.Revision)
	}
	if view.Confidence >= base.Confidence {
		t.Fatalf("confidence did not drop under pressure: base=%f got=%f", base.Confidence, view.Confidence)
	}
	if view.MaxQueueAge >= cfg.MaxQueueAge {
		t.Fatalf("max queue age did not tighten: base=%s got=%s", cfg.MaxQueueAge, view.MaxQueueAge)
	}
	if view.RetryBackoff <= cfg.RetryBackoff {
		t.Fatalf("retry backoff did not expand: base=%s got=%s", cfg.RetryBackoff, view.RetryBackoff)
	}
	if view.QueuePressurePct >= 80 {
		t.Fatalf("queue pressure threshold did not tighten: got=%d", view.QueuePressurePct)
	}
}
