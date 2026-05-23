package relay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"mevrelayv1/internal/backend"
	"mevrelayv1/internal/builder"
	"mevrelayv1/internal/config"
	"mevrelayv1/internal/metrics"
	"mevrelayv1/internal/model"
	"mevrelayv1/internal/store"
)

type Service struct {
	cfg      config.Config
	store    store.Store
	backend  backend.Simulator
	metrics  *metrics.Metrics
	queue    chan string
	clients  syncMapCounter
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

type syncMapCounter struct {
	m sync.Map
}

func (c *syncMapCounter) Inc(key string, max int) bool {
	v, _ := c.m.LoadOrStore(key, new(atomic.Int64))
	n := v.(*atomic.Int64).Add(1)
	if int(n) > max {
		v.(*atomic.Int64).Add(-1)
		return false
	}
	return true
}

func (c *syncMapCounter) Dec(key string) {
	if v, ok := c.m.Load(key); ok {
		v.(*atomic.Int64).Add(-1)
	}
}

// New constructs a relay service with bounded queueing and persistence.
func New(cfg config.Config, st store.Store, be backend.Simulator, m *metrics.Metrics) *Service {
	return &Service{
		cfg:     cfg,
		store:   st,
		backend: be,
		metrics: m,
		queue:   make(chan string, cfg.QueueDepth),
		stopCh:  make(chan struct{}),
	}
}

// Start launches the worker pool.
func (s *Service) Start(ctx context.Context) {
	for i := 0; i < s.cfg.WorkerCount; i++ {
		s.wg.Add(1)
		go s.worker(ctx)
	}
}

// Stop stops workers and waits for in-flight work to exit.
func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

// Ready reports whether the relay can safely accept work.
func (s *Service) Ready(ctx context.Context) error {
	report := s.AssessHealth(ctx)
	if !report.Ready {
		return fmt.Errorf("relay unsafe: %v", report.Reasons)
	}
	return nil
}

// Submit validates, persists, and enqueues one bundle request using an anonymous transport identity.
func (s *Service) Submit(ctx context.Context, req model.JSONRPCRequest) (model.BundleRecord, error) {
	return s.submitWithIdentity(ctx, req, "anonymous")
}

func (s *Service) submitWithIdentity(ctx context.Context, req model.JSONRPCRequest, clientID string) (model.BundleRecord, error) {
	if req.JSONRPC != "" && req.JSONRPC != "2.0" {
		return model.BundleRecord{}, fmt.Errorf("invalid jsonrpc version")
	}
	if req.Method != "" && req.Method != "eth_sendBundle" {
		return model.BundleRecord{}, fmt.Errorf("invalid method")
	}
	if len(req.Params) == 0 {
		return model.BundleRecord{}, fmt.Errorf("missing params")
	}
	p := req.Params[0]
	if len(p.Txs) == 0 {
		return model.BundleRecord{}, fmt.Errorf("missing txs")
	}

	hash := bundleHash(p)
	rec := model.BundleRecord{
		ID:         bundleID(hash, req.ID),
		BundleHash: hash,
		Request:    p,
		ClientID:   clientID,
		State:      model.StateReceived,
		Reason:     "received",
	}

	rec, err := s.store.Create(ctx, rec)
	if err != nil {
		return model.BundleRecord{}, err
	}
	s.metrics.Received.Add(1)

	rec, err = s.store.Transition(ctx, rec.ID, model.StateReceived, model.StateValidated, "validated")
	if err != nil {
		return model.BundleRecord{}, err
	}

	if !s.clients.Inc(clientID, s.cfg.MaxInFlightPerClient) {
		if rec, err = s.store.Transition(ctx, rec.ID, model.StateValidated, model.StateRejected, "client inflight limit"); err != nil {
			return model.BundleRecord{}, err
		}
		s.metrics.Rejected.Add(1)
		return rec, fmt.Errorf("client inflight limit")
	}

	rec, err = s.store.Transition(ctx, rec.ID, model.StateValidated, model.StateQueued, "queued")
	if err != nil {
		s.clients.Dec(clientID)
		return model.BundleRecord{}, err
	}

	select {
	case s.queue <- rec.ID:
		s.metrics.Queued.Add(1)
		return rec, nil
	default:
		if rec, err = s.store.Transition(ctx, rec.ID, model.StateQueued, model.StateRejected, "queue overflow"); err != nil {
			s.clients.Dec(clientID)
			return model.BundleRecord{}, err
		}
		s.metrics.QueueRejects.Add(1)
		s.metrics.Rejected.Add(1)
		s.clients.Dec(clientID)
		return rec, fmt.Errorf("queue overflow")
	}
}

// Get loads one bundle by ID.
func (s *Service) Get(ctx context.Context, id string) (model.BundleRecord, bool, error) {
	return s.store.Get(ctx, id)
}

// Snapshot returns the current records and metric snapshot.
func (s *Service) Snapshot(ctx context.Context) ([]model.BundleRecord, map[string]int64, error) {
	recs, err := s.store.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	return recs, s.metrics.Snapshot(), nil
}

func (s *Service) worker(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case id := <-s.queue:
			s.process(ctx, id)
		}
	}
}

func (s *Service) process(ctx context.Context, id string) {
	rec, ok, err := s.store.Get(ctx, id)
	if err != nil || !ok {
		return
	}
	clientID := rec.ClientID

	_, err = s.store.Transition(ctx, rec.ID, model.StateQueued, model.StateSimulating, "picked")
	if err != nil {
		return
	}

	simCtx, cancel := context.WithTimeout(ctx, s.cfg.RequestTimeout)
	defer cancel()
	start := time.Now()
	sim, err := s.backend.Simulate(simCtx, rec)
	sim.LatencyMS = time.Since(start).Milliseconds()

	if err != nil {
		s.metrics.SimulationFail.Add(1)
		if retryable(err) && rec.RetryCount < s.cfg.MaxRetries {
			if rec, err = s.store.Transition(ctx, id, model.StateSimulating, model.StateRetryPending, "transient failure"); err != nil {
				s.clients.Dec(clientID)
				return
			}
			if rec, err = s.store.UpdateRetryCount(ctx, id, rec.RetryCount+1); err != nil {
				s.clients.Dec(clientID)
				return
			}
			s.metrics.RetryPending.Add(1)
			go s.scheduleRetry(rec.ID, rec.ClientID, rec.RetryCount)
			return
		}
		if _, err = s.store.Transition(ctx, id, model.StateSimulating, model.StateDeadLetter, err.Error()); err != nil {
			s.clients.Dec(clientID)
			return
		}
		if _, err = s.store.Transition(ctx, id, model.StateDeadLetter, model.StatePersisted, "persisted dead letter"); err != nil {
			s.clients.Dec(clientID)
			return
		}
		if _, err = s.store.Transition(ctx, id, model.StatePersisted, model.StateCompleted, "completed"); err != nil {
			s.clients.Dec(clientID)
			return
		}
		s.metrics.DeadLettered.Add(1)
		s.clients.Dec(rec.ClientID)
		return
	}

	if _, err = s.store.Transition(ctx, id, model.StateSimulating, model.StateSimulated, sim.Reason); err != nil {
		s.clients.Dec(clientID)
		return
	}
	if _, err = s.store.Transition(ctx, id, model.StateSimulated, model.StateScored, "scored"); err != nil {
		s.clients.Dec(clientID)
		return
	}

	dec := builder.Decide(rec, sim)
	switch dec.Action {
	case "forward":
		if _, err = s.store.Transition(ctx, id, model.StateScored, model.StateForwarded, dec.Reason); err != nil {
			s.clients.Dec(clientID)
			return
		}
		s.metrics.Forwarded.Add(1)
	default:
		if _, err = s.store.Transition(ctx, id, model.StateScored, model.StateRejected, dec.Reason); err != nil {
			s.clients.Dec(clientID)
			return
		}
		s.metrics.Rejected.Add(1)
	}

	if _, err = s.store.UpdateResult(ctx, id, dec.Score, dec.ProfitEth, dec.Reason); err != nil {
		s.clients.Dec(clientID)
		return
	}
	if _, err = s.store.Transition(ctx, id, s.currentTerminalState(ctx, id), model.StatePersisted, "persisted"); err != nil {
		s.clients.Dec(clientID)
		return
	}
	if _, err = s.store.Transition(ctx, id, model.StatePersisted, model.StateCompleted, "completed"); err != nil {
		s.clients.Dec(clientID)
		return
	}
	s.metrics.Simulated.Add(1)
	s.clients.Dec(rec.ClientID)
}

func (s *Service) scheduleRetry(id, clientID string, retryCount int) {
	delay := s.cfg.RetryBackoff * time.Duration(retryCount)
	time.AfterFunc(delay, func() {
		ctx, cancel := context.WithTimeout(context.Background(), s.cfg.RequestTimeout)
		defer cancel()
		rec, ok, err := s.store.Get(ctx, id)
		if err != nil || !ok {
			return
		}
		if rec.State != model.StateRetryPending {
			return
		}
		if retryCount > s.cfg.MaxRetries {
			if _, err = s.store.Transition(ctx, id, model.StateRetryPending, model.StateDeadLetter, "retry exhausted"); err != nil {
				s.clients.Dec(clientID)
				return
			}
			if _, err = s.store.Transition(ctx, id, model.StateDeadLetter, model.StatePersisted, "persisted dead letter"); err != nil {
				s.clients.Dec(clientID)
				return
			}
			if _, err = s.store.Transition(ctx, id, model.StatePersisted, model.StateCompleted, "completed"); err != nil {
				s.clients.Dec(clientID)
				return
			}
			s.metrics.DeadLettered.Add(1)
			s.clients.Dec(clientID)
			return
		}
		if _, err = s.store.Transition(ctx, id, model.StateRetryPending, model.StateQueued, "retry queued"); err != nil {
			s.clients.Dec(clientID)
			return
		}
		select {
		case s.queue <- id:
			return
		default:
			if _, err = s.store.Transition(ctx, id, model.StateQueued, model.StateDeadLetter, "retry queue overflow"); err != nil {
				s.clients.Dec(clientID)
				return
			}
			if _, err = s.store.Transition(ctx, id, model.StateDeadLetter, model.StatePersisted, "persisted dead letter"); err != nil {
				s.clients.Dec(clientID)
				return
			}
			if _, err = s.store.Transition(ctx, id, model.StatePersisted, model.StateCompleted, "completed"); err != nil {
				s.clients.Dec(clientID)
				return
			}
			s.metrics.DeadLettered.Add(1)
			s.clients.Dec(clientID)
		}
	})
}

func (s *Service) currentTerminalState(ctx context.Context, id string) model.BundleState {
	rec, ok, err := s.store.Get(ctx, id)
	if err != nil || !ok {
		return model.StateRejected
	}
	switch rec.State {
	case model.StateForwarded, model.StateRejected, model.StateDeadLetter:
		return rec.State
	default:
		return model.StateRejected
	}
}

func retryable(err error) bool {
	type retryable interface{ Retryable() bool }
	var r retryable
	if errors.As(err, &r) {
		return r.Retryable()
	}
	return false
}

func bundleHash(p model.BundleRequest) string {
	h := sha256.New()
	for _, tx := range p.Txs {
		h.Write([]byte(tx))
	}
	h.Write([]byte(p.BlockNumber))
	if p.Replacement != nil {
		h.Write([]byte(*p.Replacement))
	}
	return "0x" + hex.EncodeToString(h.Sum(nil)[:16])
}

func bundleID(hash string, reqID int64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", hash, reqID)))
	return "0x" + hex.EncodeToString(h[:12])
}
