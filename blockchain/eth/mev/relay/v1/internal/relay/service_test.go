package relay

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"mevrelayv1/internal/config"
	"mevrelayv1/internal/metrics"
	"mevrelayv1/internal/model"
	"mevrelayv1/internal/store"
)

type mockBackend struct {
	mu    sync.Mutex
	mode  string
	calls int
}

func (m *mockBackend) Simulate(ctx context.Context, rec model.BundleRecord) (model.SimulationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	switch m.mode {
	case "retry":
		if m.calls == 1 {
			return model.SimulationResult{}, tempErr{msg: "temporary", retry: true}
		}
		return model.SimulationResult{ProfitEth: 0.002, Success: true, Reason: "ok"}, nil
	case "retry_always":
		return model.SimulationResult{}, tempErr{msg: "temporary", retry: true}
	case "fail":
		return model.SimulationResult{}, errors.New("permanent failure")
	default:
		return model.SimulationResult{ProfitEth: 0.002, Success: true, Reason: "ok"}, nil
	}
}

func (m *mockBackend) Ping(ctx context.Context) error {
	if m.mode == "pingfail" {
		return errors.New("backend down")
	}
	return nil
}

type tempErr struct {
	msg   string
	retry bool
}

func (e tempErr) Error() string   { return e.msg }
func (e tempErr) Retryable() bool { return e.retry }

func newTestService(t *testing.T, queueDepth, workers, retries int, mode string) (*Service, store.Store, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	svc := New(config.Config{
		QueueDepth:           queueDepth,
		WorkerCount:          workers,
		MaxRetries:           retries,
		RetryBackoff:         5 * time.Millisecond,
		MaxInFlightPerClient: 10,
		MaxPayloadBytes:      256 * 1024,
		RequestTimeout:       200 * time.Millisecond,
	}, st, &mockBackend{mode: mode}, &metrics.Metrics{})
	svc.Start(ctx)
	return svc, st, func() {
		cancel()
		svc.Stop()
	}
}

func waitForState(t *testing.T, st store.Store, id string, want model.BundleState, timeout time.Duration) model.BundleRecord {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rec, ok, err := st.Get(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if ok && rec.State == want {
			return rec
		}
		time.Sleep(5 * time.Millisecond)
	}
	rec, _, _ := st.Get(context.Background(), id)
	t.Fatalf("timeout waiting for state %s, got %s", want, rec.State)
	return rec
}

func TestSubmitAndProcessSuccess(t *testing.T) {
	svc, st, cancel := newTestService(t, 8, 1, 2, "ok")
	defer cancel()

	req := model.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "eth_sendBundle",
		Params: []model.BundleRequest{{
			Txs:         []string{"0x1", "0x2"},
			BlockNumber: "0x1",
		}},
	}

	rec, err := svc.submitWithIdentity(context.Background(), req, "client-a")
	if err != nil {
		t.Fatal(err)
	}
	waitForState(t, st, rec.ID, model.StateCompleted, time.Second)
	got, ok, err := st.Get(context.Background(), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.State != model.StateCompleted {
		t.Fatalf("expected completed, got %+v", got)
	}
	if got.State != model.StateCompleted || got.ProfitEth <= 0 {
		t.Fatalf("expected scored result")
	}
}

func TestQueueOverflowRejects(t *testing.T) {
	svc, st, cancel := newTestService(t, 0, 0, 1, "ok")
	defer cancel()

	req := model.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "eth_sendBundle",
		Params: []model.BundleRequest{{
			Txs:         []string{"0x1"},
			BlockNumber: "0x1",
		}},
	}
	rec, err := svc.submitWithIdentity(context.Background(), req, "client-a")
	if err == nil {
		t.Fatalf("expected queue overflow")
	}
	got, ok, err := st.Get(context.Background(), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.State != model.StateRejected {
		t.Fatalf("expected rejected, got %+v", got)
	}
}

func TestRetryThenSuccess(t *testing.T) {
	svc, st, cancel := newTestService(t, 8, 1, 1, "retry")
	defer cancel()

	req := model.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "eth_sendBundle",
		Params: []model.BundleRequest{{
			Txs:         []string{"0x1"},
			BlockNumber: "0x1",
		}},
	}

	rec, err := svc.submitWithIdentity(context.Background(), req, "client-b")
	if err != nil {
		t.Fatal(err)
	}
	waitForState(t, st, rec.ID, model.StateCompleted, time.Second)
	got, _, err := st.Get(context.Background(), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != model.StateCompleted {
		t.Fatalf("expected completed terminal state, got %s", got.State)
	}
	if got.RetryCount != 1 {
		t.Fatalf("expected one retry, got %d", got.RetryCount)
	}
}

func TestRetryExhaustionDeadLetter(t *testing.T) {
	svc, st, cancel := newTestService(t, 8, 1, 1, "retry_always")
	defer cancel()

	req := model.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      22,
		Method:  "eth_sendBundle",
		Params: []model.BundleRequest{{
			Txs:         []string{"0x1"},
			BlockNumber: "0x1",
		}},
	}

	rec, err := svc.submitWithIdentity(context.Background(), req, "client-b")
	if err != nil {
		t.Fatal(err)
	}
	waitForState(t, st, rec.ID, model.StateCompleted, time.Second)

	got, _, err := st.Get(context.Background(), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RetryCount != 1 {
		t.Fatalf("expected one retry before exhaustion, got %d", got.RetryCount)
	}
	if got.State != model.StateCompleted {
		t.Fatalf("expected terminal completion after dead-letter path, got state=%s reason=%s", got.State, got.Reason)
	}
}

func TestDuplicateSubmissionRejected(t *testing.T) {
	svc, _, cancel := newTestService(t, 8, 1, 1, "ok")
	defer cancel()

	req := model.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "eth_sendBundle",
		Params: []model.BundleRequest{{
			Txs:         []string{"0x1"},
			BlockNumber: "0x1",
		}},
	}
	_, err := svc.submitWithIdentity(context.Background(), req, "client-c")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.submitWithIdentity(context.Background(), req, "client-c")
	if err == nil {
		t.Fatalf("expected duplicate rejection")
	}
}

func TestInvalidRequestRejected(t *testing.T) {
	svc, _, cancel := newTestService(t, 8, 1, 1, "ok")
	defer cancel()

	cases := []model.JSONRPCRequest{
		{JSONRPC: "1.0", Method: "eth_sendBundle"},
		{JSONRPC: "2.0", Method: "bad_method"},
		{JSONRPC: "2.0", Method: "eth_sendBundle", Params: nil},
		{JSONRPC: "2.0", Method: "eth_sendBundle", Params: []model.BundleRequest{{BlockNumber: "0x1"}}},
	}

	for i, req := range cases {
		if _, err := svc.submitWithIdentity(context.Background(), req, "client-z"); err == nil {
			t.Fatalf("case %d: expected rejection", i)
		}
	}
}

func TestClientInflightLimitRejects(t *testing.T) {
	svc, st, cancel := newTestService(t, 8, 1, 1, "ok")
	defer cancel()
	svc.cfg.MaxInFlightPerClient = 0

	req := model.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      41,
		Method:  "eth_sendBundle",
		Params: []model.BundleRequest{{
			Txs:         []string{"0x1"},
			BlockNumber: "0x1",
		}},
	}
	rec, err := svc.submitWithIdentity(context.Background(), req, "client-limit")
	if err == nil {
		t.Fatalf("expected inflight limit rejection")
	}
	got, ok, err := st.Get(context.Background(), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.State != model.StateRejected {
		t.Fatalf("expected rejected state, got %+v", got)
	}
}

func TestConcurrentSubmissions(t *testing.T) {
	svc, st, cancel := newTestService(t, 64, 4, 1, "ok")
	defer cancel()

	const n = 20
	var wg sync.WaitGroup
	ids := make(chan string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := model.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      int64(i + 1),
				Method:  "eth_sendBundle",
				Params: []model.BundleRequest{{
					Txs:         []string{"0x1", "0x2"},
					BlockNumber: "0x1",
				}},
			}
			rec, err := svc.submitWithIdentity(context.Background(), req, "client-x-"+string(rune('a'+i)))
			if err != nil {
				t.Errorf("submit %d: %v", i, err)
				return
			}
			ids <- rec.ID
		}(i)
	}
	wg.Wait()
	close(ids)

	for id := range ids {
		waitForState(t, st, id, model.StateCompleted, time.Second)
	}
}
