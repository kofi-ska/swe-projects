package relay

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"mevrelayv3/internal/backend"
	"mevrelayv3/internal/broker"
	"mevrelayv3/internal/checkpoint"
	"mevrelayv3/internal/config"
	"mevrelayv3/internal/eventlog"
	"mevrelayv3/internal/graph"
	"mevrelayv3/internal/model"
	"mevrelayv3/internal/scheduler"
	state "mevrelayv3/internal/state"
	"mevrelayv3/internal/telemetry"
)

type Service struct {
	cfg         config.Config
	backend     backend.Adapter
	broker      broker.Broker
	checkpts    checkpoint.Store
	policyStore PolicyStore
	wal         *eventlog.WAL
	state       state.Store
	queue       *scheduler.Queue
	metrics     *telemetry.Metrics
	policy      *ControlPolicy
	recovery    *RecoveryController
	rollout     *RolloutController
	routing     graph.Routing
	tracer      trace.Tracer
	draining    atomic.Bool
	stopCh      chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup
	authMu      sync.RWMutex
	auth        graph.Authority
}

func New(cfg config.Config, be backend.Adapter, br broker.Broker, cp checkpoint.Store, st state.Store, wal *eventlog.WAL) *Service {
	now := time.Now().UTC()
	leaseID := fmt.Sprintf("%s-%d", cfg.ShardID, now.UnixNano())
	auth := graph.NewAuthority(cfg.ShardID, leaseID, 1, cfg.LeaseTTL, now)
	s := &Service{
		cfg:         cfg,
		backend:     be,
		broker:      br,
		checkpts:    cp,
		policyStore: newPolicyStore(cfg),
		wal:         wal,
		state:       st,
		queue:       scheduler.New(cfg.QueueDepth),
		metrics:     telemetry.New(),
		policy:      NewControlPolicy(cfg),
		recovery:    NewRecoveryController(),
		rollout:     NewRolloutController("v3"),
		routing:     graph.NewRouting(cfg.ShardSet),
		tracer:      otel.Tracer("mevrelayv3/internal/relay"),
		stopCh:      make(chan struct{}),
		auth:        auth,
	}
	seed := s.policy.Snapshot()
	s.metrics.SetPolicy(seed.Revision, seed.Pressure, seed.Confidence)
	return s
}

func (s *Service) Bootstrap(ctx context.Context) error {
	ctx, span := s.startSpan(ctx, "relay.bootstrap",
		attribute.String("shard.id", s.cfg.ShardID),
		attribute.String("region.id", s.cfg.RegionID),
	)
	defer span.End()
	if err := s.backend.Ping(ctx); err != nil {
		endSpan(span, err)
		return err
	}
	if err := s.state.Health(ctx); err != nil {
		endSpan(span, err)
		return err
	}
	if err := s.wal.Health(); err != nil {
		endSpan(span, err)
		return err
	}
	if err := s.checkpts.Health(ctx); err != nil {
		endSpan(span, err)
		return err
	}
	if err := s.policyStore.Health(ctx); err != nil {
		endSpan(span, err)
		return err
	}
	if err := s.state.SetAuthority(ctx, s.currentAuthority()); err != nil {
		endSpan(span, err)
		return err
	}
	s.recovery.Validate("bootstrap complete")
	if snap, ok, err := s.policyStore.Load(ctx, s.cfg.ShardID); err != nil {
		endSpan(span, err)
		return err
	} else if ok {
		s.policy.ApplySnapshot(snap)
		s.metrics.SetPolicy(snap.Revision, snap.Pressure, snap.Confidence)
	}
	return nil
}

func (s *Service) Start(ctx context.Context) {
	for i := 0; i < s.cfg.WorkerCount; i++ {
		s.wg.Add(1)
		go s.worker(ctx)
	}
	s.wg.Add(1)
	go s.retryLoop(ctx)
	s.wg.Add(1)
	go s.authorityLoop(ctx)
	s.wg.Add(1)
	go s.policyLoop(ctx)
}

func (s *Service) Stop() {
	s.stopOnce.Do(func() {
		s.draining.Store(true)
		s.rollout.BeginDrain("shutdown")
		close(s.stopCh)
		s.queue.Close()
	})
	s.wg.Wait()
	_ = s.policyStore.Close()
}

func (s *Service) Drain() {
	s.draining.Store(true)
	s.rollout.BeginDrain("operator drain")
}

func (s *Service) Ready(ctx context.Context) error {
	ctx, span := s.startSpan(ctx, "relay.ready",
		attribute.String("shard.id", s.cfg.ShardID),
		attribute.String("region.id", s.cfg.RegionID),
	)
	defer span.End()
	report := s.AssessHealth(ctx)
	if !report.Ready {
		endSpan(span, fmt.Errorf("relay unsafe: %v", report.Reasons))
		return fmt.Errorf("relay unsafe: %v", report.Reasons)
	}
	if s.draining.Load() {
		endSpan(span, fmt.Errorf("relay draining"))
		return fmt.Errorf("relay draining")
	}
	if !s.rollout.Ready() {
		endSpan(span, ErrRolloutBlocked)
		return ErrRolloutBlocked
	}
	if !s.recovery.Ready() {
		endSpan(span, ErrQuarantined)
		return ErrQuarantined
	}
	return nil
}

func (s *Service) Submit(ctx context.Context, req model.JSONRPCRequest) (model.BundleRecord, error) {
	return s.submitWithIdentity(ctx, req, "anonymous", s.cfg.RegionID)
}

func (s *Service) SubmitWithIdentity(ctx context.Context, req model.JSONRPCRequest, clientID, regionID string) (model.BundleRecord, error) {
	return s.submitWithIdentity(ctx, req, clientID, regionID)
}

func (s *Service) submitWithIdentity(ctx context.Context, req model.JSONRPCRequest, clientID, regionID string) (model.BundleRecord, error) {
	ctx, span := s.startSpan(ctx, "relay.submit",
		attribute.String("region.id", regionID),
		attribute.String("shard.id", s.cfg.ShardID),
		attribute.String("client.id", clientID),
	)
	defer span.End()
	s.metrics.IncSubmitted()
	if s.draining.Load() {
		endSpan(span, ErrQueueDisabled)
		return model.BundleRecord{}, ErrQueueDisabled
	}
	if !s.rollout.AllowWrites() {
		endSpan(span, ErrRolloutBlocked)
		return model.BundleRecord{}, ErrRolloutBlocked
	}
	if !s.recovery.AllowWrites() {
		endSpan(span, ErrQuarantined)
		return model.BundleRecord{}, ErrQuarantined
	}
	if err := validateBundleRequest(req); err != nil {
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	p := req.Params[0]
	hash := bundleHash(p)
	if route := s.routing.Route(bundleKey(p, s.cfg.NetworkID)); route != "" && route != s.cfg.ShardID {
		s.metrics.IncWrongShard()
		endSpan(span, ErrWrongShard)
		return model.BundleRecord{}, ErrWrongShard
	}
	auth := s.currentAuthority()
	if !auth.Valid(time.Now().UTC()) {
		s.metrics.IncStaleAuthority()
		endSpan(span, ErrStaleAuthority)
		return model.BundleRecord{}, ErrStaleAuthority
	}
	policy := s.policy.Snapshot()
	if policy.Confidence < policy.ConfidenceFloor*0.5 {
		s.metrics.IncRejected()
		endSpan(span, ErrLowControlConfidence)
		return model.BundleRecord{}, ErrLowControlConfidence
	}
	decision := s.scoreAdmission(model.BundleRecord{Request: p})
	rec := model.BundleRecord{
		ID:                bundleID(hash, req.ID),
		BundleHash:        hash,
		Request:           p,
		ClientID:          clientID,
		RegionID:          regionID,
		ShardID:           s.cfg.ShardID,
		LeaseID:           auth.LeaseID,
		LeaseEpoch:        auth.Epoch,
		FenceToken:        auth.FenceToken,
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
		if err == state.ErrDuplicateBundle {
			s.metrics.IncDuplicate()
		}
		endSpan(span, err)
		return rec, err
	}
	rec, err = s.state.TransitionBundle(ctx, rec.ID, model.StateReceived, model.StateValidated, "validated")
	if err != nil {
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	if err := s.emitTransition(ctx, rec, model.StateReceived, model.StateValidated, "validated"); err != nil {
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	if !decision.accepted {
		s.metrics.IncRejected()
		result, err := s.rejectBundle(ctx, rec.ID, model.StateValidated, decision.reason)
		endSpan(span, err)
		return result, err
	}
	if _, err := s.state.ReserveInflight(ctx, clientID, s.cfg.MaxInFlightPerClient); err != nil {
		s.metrics.IncInflightLimit()
		result, rejErr := s.rejectBundle(ctx, rec.ID, model.StateValidated, "client inflight limit")
		endSpan(span, rejErr)
		return result, rejErr
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
		result, rejErr := s.rejectBundle(ctx, rec.ID, model.StateValidated, "queue overflow")
		endSpan(span, rejErr)
		return result, rejErr
	}
	s.metrics.IncAccepted()
	if evicted != nil {
		if evictedRec, ok, err := s.state.GetBundle(ctx, evicted.ID); err == nil && ok {
			if evictedRec.State == model.StateQueued {
				if evictedRec, terr := s.state.TransitionBundle(ctx, evicted.ID, model.StateQueued, model.StateRejected, "priority eviction"); terr == nil {
					if err := s.emitTransition(ctx, evictedRec, model.StateQueued, model.StateRejected, "priority eviction"); err == nil {
						_, _ = s.finishTerminal(ctx, evictedRec, model.StateRejected, "priority eviction")
					}
				}
			}
		}
	}
	rec, err = s.state.TransitionBundle(ctx, rec.ID, model.StateValidated, model.StateQueued, "queued")
	if err != nil {
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	if err := s.emitTransition(ctx, rec, model.StateValidated, model.StateQueued, "queued"); err != nil {
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	return rec, nil
}

func (s *Service) Get(ctx context.Context, id string) (model.BundleRecord, bool, error) {
	return s.state.GetBundle(ctx, id)
}

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

func (s *Service) MetricsSnapshot() telemetry.Snapshot { return s.metrics.Snapshot() }

func (s *Service) PolicySnapshot() PolicySnapshot { return s.policy.Snapshot() }

func (s *Service) RecoverySnapshot() RecoverySnapshot { return s.recovery.Snapshot() }

func (s *Service) RolloutSnapshot() RolloutSnapshot { return s.rollout.Snapshot() }

func (s *Service) Recover(ctx context.Context) error {
	ctx, span := s.startSpan(ctx, "relay.recover",
		attribute.String("shard.id", s.cfg.ShardID),
		attribute.String("region.id", s.cfg.RegionID),
	)
	defer span.End()
	s.recovery.BeginReplay("operator requested recovery")
	recs, cps, err := s.Snapshot(ctx)
	if err != nil {
		s.recovery.Quarantine(err.Error())
		endSpan(span, err)
		return err
	}
	if len(recs) == 0 && len(cps) == 0 {
		s.recovery.Validate("empty store")
		endSpan(span, nil)
		return nil
	}
	if len(cps) == 0 {
		err = ErrQuarantined
		s.recovery.Quarantine("checkpoint missing")
		endSpan(span, err)
		return err
	}
	last := cps[len(cps)-1]
	if last.ShardID != s.cfg.ShardID || last.Root == "" || last.BundleID == "" {
		err = ErrQuarantined
		s.recovery.Quarantine("checkpoint mismatch")
		endSpan(span, err)
		return err
	}
	s.recovery.BeginValidation("checkpoint validated")
	s.recovery.Validate("rejoin complete")
	endSpan(span, nil)
	return nil
}

func (s *Service) currentAuthority() graph.Authority {
	s.authMu.RLock()
	defer s.authMu.RUnlock()
	return s.auth
}

func (s *Service) renewAuthority(ctx context.Context) {
	ctx, span := s.startSpan(ctx, "relay.authority.renew",
		attribute.String("shard.id", s.cfg.ShardID),
		attribute.String("region.id", s.cfg.RegionID),
	)
	defer span.End()
	s.authMu.Lock()
	s.auth = s.auth.Renew(s.cfg.LeaseTTL, time.Now().UTC())
	auth := s.auth
	s.authMu.Unlock()
	if err := s.state.SetAuthority(ctx, auth); err != nil {
		s.metrics.IncStaleAuthority()
		endSpan(span, err)
	}
}

func (s *Service) authorityValid() bool {
	return s.currentAuthority().Valid(time.Now().UTC())
}

func (s *Service) policyLoop(ctx context.Context) {
	defer s.wg.Done()
	interval := s.cfg.LeaseRenewInterval
	if interval <= 0 {
		interval = time.Second
	}
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			prev := s.policy.Snapshot()
			report := s.AssessHealth(ctx)
			snap := s.metrics.Snapshot()
			view := s.policy.Adapt(s.cfg, snap, report)
			s.metrics.SetPolicy(view.Revision, view.Pressure, view.Confidence)
			if view.Revision != prev.Revision {
				s.metrics.IncPolicyAdjustment()
			}
			_ = s.policyStore.Save(ctx, s.cfg.ShardID, view)
		}
	}
}
