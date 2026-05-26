package relay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mevrelayv2/internal/backend/local"
	"mevrelayv2/internal/broker/memory"
	"mevrelayv2/internal/config"
	"mevrelayv2/internal/eventlog"
	"mevrelayv2/internal/model"
	stateMemory "mevrelayv2/internal/state/memory"
)

func TestDrainBlocksReadinessAndAdmissions(t *testing.T) {
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

	svc.Drain()
	report := svc.AssessHealth(context.Background())
	if report.State != HealthStateDraining {
		t.Fatalf("expected draining health state, got %+v", report)
	}
	if err := svc.Ready(context.Background()); err == nil || !strings.Contains(err.Error(), "draining") {
		t.Fatalf("expected draining readiness error, got %v", err)
	}
	if _, err := svc.Submit(context.Background(), defaultRequest()); err == nil || err != ErrQueueDisabled {
		t.Fatalf("expected queue disabled on drain, got %v", err)
	}
}

func TestAuthRequiredWhenConfigured(t *testing.T) {
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
		APIAuthToken:   "secret",
	}, local.New(), memory.New(8), stateMemory.New(), wal)

	h := Handler{Svc: svc}
	req := httptest.NewRequest(http.MethodPost, "/relay/v2/bundle", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"eth_sendBundle","params":[{"txs":["0x1"],"blockNumber":"0x1"}]}`))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", rr.Code)
	}
}

func defaultRequest() model.JSONRPCRequest {
	return model.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "eth_sendBundle",
		Params: []model.BundleRequest{{
			Txs:         []string{"0x1"},
			BlockNumber: "0x1",
		}},
	}
}
