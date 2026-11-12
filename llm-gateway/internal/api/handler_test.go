package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kofi-ska/llm-gateway/internal/observability"
	"github.com/kofi-ska/llm-gateway/internal/pipeline"
	"github.com/kofi-ska/llm-gateway/internal/worker"
	"github.com/prometheus/client_golang/prometheus"
)

func testHandler(t *testing.T, started bool, timeout time.Duration) (*Handler, *pipeline.Pipeline) {
	t.Helper()
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg, 4)
	p := pipeline.New(4, 2, 1, time.Millisecond, worker.Worker{}, m)
	if !started {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := p.Shutdown(ctx); err != nil {
			t.Fatal(err)
		}
	}
	return &Handler{Pipeline: p, Metrics: m, Timeout: timeout, WriteTimeout: time.Second}, p
}

func stopPipeline(p *pipeline.Pipeline) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = p.Shutdown(ctx)
}

func TestValidation(t *testing.T) {
	h, p := testHandler(t, true, time.Second)
	defer stopPipeline(p)
	cases := []string{`{"prompt":"","max_tokens":1,"priority":"normal"}`, `{"prompt":"ok","max_tokens":0,"priority":"normal"}`, `{"prompt":"ok","max_tokens":1,"priority":"urgent"}`, `{"prompt":"ok","max_tokens":1,"priority":"normal"} {}`}
	for _, body := range cases {
		rr := httptest.NewRecorder()
		h.Generate(rr, httptest.NewRequest(http.MethodPost, route, strings.NewReader(body)))
		if rr.Code != http.StatusBadRequest {
			t.Errorf("body %s: status=%d", body, rr.Code)
		}
	}
}

func TestStreamingResponse(t *testing.T) {
	h, p := testHandler(t, true, time.Second)
	defer stopPipeline(p)
	rr := httptest.NewRecorder()
	h.Generate(rr, httptest.NewRequest(http.MethodPost, route, strings.NewReader(`{"prompt":"hello","max_tokens":3,"priority":"normal"}`)))
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/x-ndjson" {
		t.Fatalf("content type=%s", got)
	}
	lines := strings.Split(strings.TrimSpace(rr.Body.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("lines=%d body=%s", len(lines), rr.Body.String())
	}
	var done streamEvent
	if err := json.Unmarshal([]byte(lines[3]), &done); err != nil || !done.Done || done.Tokens != 3 {
		t.Fatalf("bad done event: %s", lines[3])
	}
}

func TestUnavailablePipelineReturnsOverload(t *testing.T) {
	h, _ := testHandler(t, false, time.Second)
	rr := httptest.NewRecorder()
	h.Generate(rr, httptest.NewRequest(http.MethodPost, route, strings.NewReader(`{"prompt":"hello","max_tokens":1,"priority":"normal"}`)))
	if rr.Code != http.StatusServiceUnavailable || rr.Header().Get("Retry-After") == "" {
		t.Fatalf("status=%d headers=%v", rr.Code, rr.Header())
	}
}

func TestTimeoutStopsRequest(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := observability.NewMetrics(reg, 1)
	p := pipeline.New(1, 1, 1, time.Millisecond, worker.Worker{BaseDelay: time.Second}, m)
	defer stopPipeline(p)
	h := &Handler{Pipeline: p, Metrics: m, Timeout: 5 * time.Millisecond, WriteTimeout: time.Second}
	rr := httptest.NewRecorder()
	h.Generate(rr, httptest.NewRequest(http.MethodPost, route, strings.NewReader(`{"prompt":"hello","max_tokens":1,"priority":"normal"}`)))
	if rr.Body.Len() != 0 {
		t.Fatalf("expected terminated stream, got %s", rr.Body.String())
	}
}

func TestClientCancellation(t *testing.T) {
	h, p := testHandler(t, true, time.Second)
	defer stopPipeline(p)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rr := httptest.NewRecorder()
	h.Generate(rr, httptest.NewRequest(http.MethodPost, route, strings.NewReader(`{"prompt":"hello","max_tokens":1,"priority":"normal"}`)).WithContext(ctx))
}

func BenchmarkResponseSerialization(b *testing.B) {
	e := streamEvent{Token: " distributed"}
	for i := 0; i < b.N; i++ {
		var out bytes.Buffer
		_ = json.NewEncoder(&out).Encode(e)
	}
}
