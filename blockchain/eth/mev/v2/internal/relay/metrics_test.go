package relay

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mevrelayv2/internal/backend/local"
	"mevrelayv2/internal/broker/memory"
	"mevrelayv2/internal/config"
	"mevrelayv2/internal/eventlog"
	stateMemory "mevrelayv2/internal/state/memory"
)

func TestMetricsRenderAndHealthSync(t *testing.T) {
	dir := t.TempDir()
	wal, err := eventlog.Open(filepath.Join(dir, "wal.jsonl"), 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer wal.Close()

	svc := New(config.Config{
		RegionID:       "local",
		BrokerTopic:    "mev.v2.events",
		BrokerBuffer:   8,
		QueueDepth:     4,
		WorkerCount:    1,
		RequestTimeout: 100 * time.Millisecond,
		MaxQueueAge:    50 * time.Millisecond,
	}, local.New(), memory.New(8), stateMemory.New(), wal)

	report := svc.AssessHealth(context.Background())
	if report.RegionID != "local" {
		t.Fatalf("unexpected region: %+v", report)
	}

	body := svc.metrics.RenderPrometheus()
	if !strings.Contains(body, "mevrelay_queue_depth") {
		t.Fatalf("missing queue depth metric: %s", body)
	}
	if !strings.Contains(body, "mevrelay_health_state") {
		t.Fatalf("missing health state metric: %s", body)
	}
}
