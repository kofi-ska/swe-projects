package relay

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"mevrelayv2/internal/backend"
	"mevrelayv2/internal/broker"
	"mevrelayv2/internal/config"
	"mevrelayv2/internal/eventlog"
	"mevrelayv2/internal/model"
	"mevrelayv2/internal/scheduler"
	coordstate "mevrelayv2/internal/state"
	"mevrelayv2/internal/telemetry"
)

// Service coordinates submission, eventing, and bounded processing.
type Service struct {
	cfg     config.Config
	backend backend.Adapter
	broker  broker.Broker
	wal     *eventlog.WAL
	state   coordstate.Store

	queue    *scheduler.Queue
	metrics  *telemetry.Metrics
	draining atomic.Bool
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// New constructs a v2 relay service with bounded queueing and append-only evidence.
func New(cfg config.Config, be backend.Adapter, br broker.Broker, st coordstate.Store, wal *eventlog.WAL) *Service {
	return &Service{
		cfg:     cfg,
		backend: be,
		broker:  br,
		wal:     wal,
		state:   st,
		queue:   scheduler.New(cfg.QueueDepth),
		metrics: telemetry.New(),
		stopCh:  make(chan struct{}),
	}
}

// Bootstrap validates the external state backend.
func (s *Service) Bootstrap(ctx context.Context) error {
	if err := s.backend.Ping(ctx); err != nil {
		return err
	}
	if err := s.state.Health(ctx); err != nil {
		return err
	}
	return s.wal.Health()
}

// Start launches the worker pool.
func (s *Service) Start(ctx context.Context) {
	for i := 0; i < s.cfg.WorkerCount; i++ {
		s.wg.Add(1)
		go s.worker(ctx)
	}
	s.wg.Add(1)
	go s.retryLoop(ctx)
}

// Stop terminates workers and waits for them to exit.
func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		s.draining.Store(true)
		close(s.stopCh)
		s.queue.Close()
	})
	s.wg.Wait()
}

// Drain marks the relay as draining and stops new admissions.
func (s *Service) Drain() {
	s.draining.Store(true)
}

// Ready reports whether the relay can safely accept work.
func (s *Service) Ready(ctx context.Context) error {
	report := s.AssessHealth(ctx)
	if !report.Ready {
		return fmt.Errorf("relay unsafe: %v", report.Reasons)
	}
	if s.draining.Load() {
		return fmt.Errorf("relay draining")
	}
	return nil
}

// Submit validates and enqueues one bundle request.
func (s *Service) Submit(ctx context.Context, req model.JSONRPCRequest) (model.BundleRecord, error) {
	return s.submitWithIdentity(ctx, req, "anonymous", s.cfg.RegionID)
}

func (s *Service) submitWithIdentity(ctx context.Context, req model.JSONRPCRequest, clientID, regionID string) (model.BundleRecord, error) {
	s.metrics.IncSubmitted()
	if s.draining.Load() {
		return model.BundleRecord{}, ErrQueueDisabled
	}
	if err := validateBundleRequest(req); err != nil {
		return model.BundleRecord{}, err
	}
	p := req.Params[0]
	decision := s.scoreAdmission(model.BundleRecord{Request: p})

	rec := model.BundleRecord{
		ID:                bundleID(bundleHash(p), req.ID),
		BundleHash:        bundleHash(p),
		Request:           p,
		ClientID:          clientID,
		RegionID:          regionID,
		State:             model.StateReceived,
		Reason:            "received",
		Version:           1,
		Sequence:          1,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
		DeadlineAt:        decision.deadline,
		ExpectedValue:     decision.value,
		ExpectedCost:      decision.cost,
		ExpectedServiceMS: decision.serviceMS,
		Priority:          decision.priority,
	}

	var err error
	rec, err = s.state.CreateBundle(ctx, rec)
	if err != nil {
		if err == coordstate.ErrDuplicateBundle {
			s.metrics.IncDuplicate()
		}
		return rec, err
	}

	rec, err = s.state.TransitionBundle(ctx, rec.ID, model.StateReceived, model.StateValidated, "validated")
	if err != nil {
		return model.BundleRecord{}, err
	}
	if err := s.emitTransition(ctx, rec, model.StateReceived, model.StateValidated, "validated"); err != nil {
		return model.BundleRecord{}, err
	}

	if !decision.accepted {
		s.metrics.IncRejected()
		return s.rejectBundle(ctx, rec.ID, model.StateValidated, decision.reason)
	}

	if _, err := s.state.ReserveInflight(ctx, clientID, s.cfg.MaxInFlightPerClient); err != nil {
		s.metrics.IncInflightLimit()
		return s.rejectBundle(ctx, rec.ID, model.StateValidated, "client inflight limit")
	}

	evicted, accepted, err := s.queue.Push(scheduler.Item{
		ID:                rec.ID,
		Priority:          rec.Priority,
		DeadlineAt:        rec.DeadlineAt,
		EnqueuedAt:        time.Now().UTC(),
		ExpectedValue:     rec.ExpectedValue,
		ExpectedCost:      rec.ExpectedCost,
		ExpectedServiceMS: rec.ExpectedServiceMS,
		Reason:            "admitted",
	})
	if err != nil || !accepted {
		s.metrics.IncQueueOverflow()
		return s.rejectBundle(ctx, rec.ID, model.StateValidated, "queue overflow")
	}
		s.metrics.IncAccepted()
		if evicted != nil {
			if _, ok, err := s.state.GetBundle(ctx, evicted.ID); err == nil && ok {
				if evictedRec, terr := s.state.TransitionBundle(ctx, evicted.ID, model.StateQueued, model.StateRejected, "priority eviction"); terr == nil {
					if err := s.emitTransition(ctx, evictedRec, model.StateQueued, model.StateRejected, "priority eviction"); err == nil {
						_, _ = s.finishTerminal(ctx, evictedRec, model.StateRejected, "priority eviction")
					}
				}
			}
		}

	rec, err = s.state.TransitionBundle(ctx, rec.ID, model.StateValidated, model.StateQueued, "queued")
	if err != nil {
		return model.BundleRecord{}, err
	}
	if err := s.emitTransition(ctx, rec, model.StateValidated, model.StateQueued, "queued"); err != nil {
		return model.BundleRecord{}, err
	}
	return rec, nil
}

// Get loads one bundle by ID.
func (s *Service) Get(ctx context.Context, id string) (model.BundleRecord, bool, error) {
	return s.state.GetBundle(ctx, id)
}

// Snapshot returns current bundles and checkpoints.
func (s *Service) Snapshot(ctx context.Context) ([]model.BundleRecord, []model.CheckpointRecord, error) {
	recs, err := s.state.ListBundles(ctx, s.cfg.HistoryLimit)
	if err != nil {
		return nil, nil, err
	}
	cps, err := s.state.ListCheckpoints(ctx, s.cfg.HistoryLimit)
	if err != nil {
		return nil, nil, err
	}
	return recs, cps, nil
}
