package relay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"mevrelayv2/internal/backend"
	"mevrelayv2/internal/broker"
	"mevrelayv2/internal/commitment"
	"mevrelayv2/internal/config"
	"mevrelayv2/internal/eventlog"
	"mevrelayv2/internal/model"
	"mevrelayv2/internal/scheduler"
	coordstate "mevrelayv2/internal/state"
)

// Service coordinates submission, eventing, and bounded processing.
type Service struct {
	cfg     config.Config
	backend backend.Adapter
	broker  broker.Broker
	wal     *eventlog.WAL
	state   coordstate.Store

	queue    *scheduler.Queue
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
		close(s.stopCh)
		s.queue.Close()
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

// Submit validates and enqueues one bundle request.
func (s *Service) Submit(ctx context.Context, req model.JSONRPCRequest) (model.BundleRecord, error) {
	return s.submitWithIdentity(ctx, req, "anonymous", s.cfg.RegionID)
}

func (s *Service) submitWithIdentity(ctx context.Context, req model.JSONRPCRequest, clientID, regionID string) (model.BundleRecord, error) {
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
		if _, terr := s.state.TransitionBundle(ctx, rec.ID, model.StateValidated, model.StateRejected, decision.reason); terr != nil {
			return model.BundleRecord{}, terr
		}
		if err := s.emitTransition(ctx, rec, model.StateValidated, model.StateRejected, decision.reason); err != nil {
			return model.BundleRecord{}, err
		}
		if err := s.finishTerminal(ctx, rec.ID, model.StateRejected, decision.reason); err != nil {
			return model.BundleRecord{}, err
		}
		rec, _, err = s.state.GetBundle(ctx, rec.ID)
		return rec, err
	}

	if _, err := s.state.ReserveInflight(ctx, clientID, s.cfg.MaxInFlightPerClient); err != nil {
		if _, terr := s.state.TransitionBundle(ctx, rec.ID, model.StateValidated, model.StateRejected, "client inflight limit"); terr != nil {
			return model.BundleRecord{}, terr
		}
		if err := s.emitTransition(ctx, rec, model.StateValidated, model.StateRejected, "client inflight limit"); err != nil {
			return model.BundleRecord{}, err
		}
		if err := s.finishTerminal(ctx, rec.ID, model.StateRejected, "client inflight limit"); err != nil {
			return model.BundleRecord{}, err
		}
		rec, _, err = s.state.GetBundle(ctx, rec.ID)
		return rec, err
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
		if _, terr := s.state.TransitionBundle(ctx, rec.ID, model.StateValidated, model.StateRejected, "queue overflow"); terr != nil {
			return model.BundleRecord{}, terr
		}
		if err := s.emitTransition(ctx, rec, model.StateValidated, model.StateRejected, "queue overflow"); err != nil {
			return model.BundleRecord{}, err
		}
		if err := s.finishTerminal(ctx, rec.ID, model.StateRejected, "queue overflow"); err != nil {
			return model.BundleRecord{}, err
		}
		rec, _, err = s.state.GetBundle(ctx, rec.ID)
		return rec, err
	}
	if evicted != nil {
		if ev, ok, err := s.state.GetBundle(ctx, evicted.ID); err == nil && ok {
			if _, terr := s.state.TransitionBundle(ctx, evicted.ID, model.StateQueued, model.StateRejected, "priority eviction"); terr == nil {
				if err := s.emitTransition(ctx, ev, model.StateQueued, model.StateRejected, "priority eviction"); err == nil {
					_ = s.finishTerminal(ctx, evicted.ID, model.StateRejected, "priority eviction")
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

type admissionDecision struct {
	value     float64
	cost      float64
	serviceMS int64
	deadline  time.Time
	slack     time.Duration
	priority  float64
	reason    string
	accepted  bool
}

func (s *Service) scoreAdmission(rec model.BundleRecord) admissionDecision {
	now := time.Now().UTC()
	deadlineAt := time.Unix(rec.Request.MaxTimestamp, 0).UTC()
	if rec.Request.MaxTimestamp <= 0 {
		deadlineAt = now.Add(s.cfg.RequestTimeout)
	}
	count := len(rec.Request.Txs)
	value := float64(count) * s.cfg.ValuePerTx
	serviceMS := s.estimateServiceMS(count)
	cost := float64(serviceMS) * s.cfg.CostPerMS
	slack := deadlineAt.Sub(now)
	net := value - cost
	if serviceMS <= 0 {
		serviceMS = 1
	}
	priority := (net / float64(serviceMS)) + slack.Seconds()
	accepted := true
	reason := "admitted"
	if slack <= s.cfg.MinDeadlineSlack {
		accepted = false
		reason = "stale deadline"
	}
	if serviceMS > int64(slack/time.Millisecond) {
		accepted = false
		reason = "insufficient slack"
	}
	if net < s.cfg.MinNetValue {
		accepted = false
		reason = "negative net value"
	}
	return admissionDecision{
		value:     value,
		cost:      cost,
		serviceMS: serviceMS,
		deadline:  deadlineAt,
		slack:     slack,
		priority:  priority,
		reason:    reason,
		accepted:  accepted,
	}
}

func (s *Service) estimateServiceMS(txCount int) int64 {
	base := int64(25)
	perTx := int64(5)
	switch strings.ToLower(s.cfg.BackendKind) {
	case string(backend.KindAnvil):
		base = 40
		perTx = 8
	case string(backend.KindSepolia):
		base = 120
		perTx = 14
	case string(backend.KindLocal):
		base = 10
		perTx = 3
	}
	if txCount < 0 {
		txCount = 0
	}
	return base + perTx*int64(txCount)
}

func (s *Service) worker(ctx context.Context) {
	defer s.wg.Done()
	for {
		item, ok, err := s.queue.Pop(ctx)
		if err != nil || !ok {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		default:
		}
		s.process(ctx, item)
	}
}

func (s *Service) retryLoop(ctx context.Context) {
	defer s.wg.Done()
	interval := s.cfg.RetryBackoff / 2
	if interval <= 0 {
		interval = 50 * time.Millisecond
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
			s.drainRetries(ctx)
		}
	}
}

func (s *Service) drainRetries(ctx context.Context) {
	for {
		ids, err := s.state.ClaimDueRetries(ctx, time.Now().UTC(), 256)
		if err != nil || len(ids) == 0 {
			return
		}
		for _, id := range ids {
			s.processRetry(ctx, id)
		}
	}
}

func (s *Service) processRetry(ctx context.Context, id string) {
	rec, ok, err := s.state.GetBundle(ctx, id)
	if err != nil || !ok {
		return
	}
	if rec.State != model.StateRetryPending {
		return
	}
	if rec.RetryCount > s.cfg.MaxRetries {
		if _, err := s.state.TransitionBundle(ctx, id, model.StateRetryPending, model.StateDeadLetter, "retry exhausted"); err != nil {
			return
		}
		rec, _, err = s.state.GetBundle(ctx, id)
		if err != nil {
			return
		}
		if err := s.emitTransition(ctx, rec, model.StateRetryPending, model.StateDeadLetter, "retry exhausted"); err != nil {
			return
		}
		_ = s.finishTerminal(ctx, id, model.StateDeadLetter, "retry exhausted")
		return
	}
	if _, err := s.state.TransitionBundle(ctx, id, model.StateRetryPending, model.StateQueued, "retry queued"); err != nil {
		return
	}
	rec, _, err = s.state.GetBundle(ctx, id)
	if err != nil {
		return
	}
	if err := s.emitTransition(ctx, rec, model.StateRetryPending, model.StateQueued, "retry queued"); err != nil {
		return
	}
	evicted, accepted, pushErr := s.queue.Push(scheduler.Item{
		ID:                id,
		Priority:          rec.Priority,
		DeadlineAt:        rec.DeadlineAt,
		EnqueuedAt:        time.Now().UTC(),
		ExpectedValue:     rec.ExpectedValue,
		ExpectedCost:      rec.ExpectedCost,
		ExpectedServiceMS: rec.ExpectedServiceMS,
		Reason:            "retry queued",
	})
	if pushErr != nil || !accepted {
		if _, err := s.state.TransitionBundle(ctx, id, model.StateQueued, model.StateDeadLetter, "retry queue overflow"); err != nil {
			return
		}
		rec, _, err = s.state.GetBundle(ctx, id)
		if err != nil {
			return
		}
		if err := s.emitTransition(ctx, rec, model.StateQueued, model.StateDeadLetter, "retry queue overflow"); err != nil {
			return
		}
		_ = s.finishTerminal(ctx, id, model.StateDeadLetter, "retry queue overflow")
		return
	}
	if evicted != nil {
		if ev, ok, err := s.state.GetBundle(ctx, evicted.ID); err == nil && ok {
			if _, terr := s.state.TransitionBundle(ctx, evicted.ID, model.StateQueued, model.StateRejected, "priority eviction"); terr == nil {
				if err := s.emitTransition(ctx, ev, model.StateQueued, model.StateRejected, "priority eviction"); err == nil {
					_ = s.finishTerminal(ctx, evicted.ID, model.StateRejected, "priority eviction")
				}
			}
		}
	}
}

func (s *Service) process(ctx context.Context, item scheduler.Item) {
	rec, ok, err := s.state.GetBundle(ctx, item.ID)
	if err != nil || !ok {
		return
	}

	now := time.Now().UTC()
	if !rec.DeadlineAt.IsZero() && now.After(rec.DeadlineAt) {
		if _, err := s.state.TransitionBundle(ctx, rec.ID, model.StateQueued, model.StateRejected, "expired in queue"); err != nil {
			return
		}
		if err := s.emitTransition(ctx, rec, model.StateQueued, model.StateRejected, "expired in queue"); err != nil {
			return
		}
		_ = s.finishTerminal(ctx, rec.ID, model.StateRejected, "expired in queue")
		return
	}
	if !rec.QueuedAt.IsZero() && now.Sub(rec.QueuedAt) > s.cfg.MaxQueueAge {
		if _, err := s.state.TransitionBundle(ctx, rec.ID, model.StateQueued, model.StateRejected, "queue age exceeded"); err != nil {
			return
		}
		if err := s.emitTransition(ctx, rec, model.StateQueued, model.StateRejected, "queue age exceeded"); err != nil {
			return
		}
		_ = s.finishTerminal(ctx, rec.ID, model.StateRejected, "queue age exceeded")
		return
	}
	if rec.ExpectedValue-rec.ExpectedCost < s.cfg.MinNetValue {
		if _, err := s.state.TransitionBundle(ctx, rec.ID, model.StateQueued, model.StateRejected, "insufficient priority"); err != nil {
			return
		}
		if err := s.emitTransition(ctx, rec, model.StateQueued, model.StateRejected, "insufficient priority"); err != nil {
			return
		}
		_ = s.finishTerminal(ctx, rec.ID, model.StateRejected, "insufficient priority")
		return
	}
	if !rec.DeadlineAt.IsZero() && time.Duration(rec.ExpectedServiceMS)*time.Millisecond > time.Until(rec.DeadlineAt) {
		if _, err := s.state.TransitionBundle(ctx, rec.ID, model.StateQueued, model.StateRejected, "insufficient slack"); err != nil {
			return
		}
		if err := s.emitTransition(ctx, rec, model.StateQueued, model.StateRejected, "insufficient slack"); err != nil {
			return
		}
		_ = s.finishTerminal(ctx, rec.ID, model.StateRejected, "insufficient slack")
		return
	}

	if _, err := s.state.TransitionBundle(ctx, rec.ID, model.StateQueued, model.StateSimulating, "picked"); err != nil {
		return
	}
	rec, _, err = s.state.GetBundle(ctx, item.ID)
	if err != nil {
		return
	}
	if err := s.emitTransition(ctx, rec, model.StateQueued, model.StateSimulating, "picked"); err != nil {
		return
	}

	simCtx, cancel := context.WithTimeout(ctx, s.cfg.RequestTimeout)
	defer cancel()
	result, err := s.backend.Simulate(simCtx, rec)

	if err != nil {
		if retryable(err) && rec.RetryCount < s.cfg.MaxRetries {
			if _, err := s.state.TransitionBundle(ctx, rec.ID, model.StateSimulating, model.StateRetryPending, "transient failure"); err != nil {
				return
			}
			rec, _, err = s.state.GetBundle(ctx, rec.ID)
			if err != nil {
				return
			}
			if err := s.emitTransition(ctx, rec, model.StateSimulating, model.StateRetryPending, "transient failure"); err != nil {
				return
			}
			if _, err := s.state.UpdateRetryCount(ctx, rec.ID, rec.RetryCount+1); err != nil {
				return
			}
			if err := s.scheduleRetry(rec.ID, rec.RetryCount+1); err != nil {
				if _, terr := s.state.TransitionBundle(ctx, rec.ID, model.StateRetryPending, model.StateDeadLetter, "retry schedule failed"); terr != nil {
					return
				}
				rec, _, err = s.state.GetBundle(ctx, rec.ID)
				if err != nil {
					return
				}
				if err := s.emitTransition(ctx, rec, model.StateRetryPending, model.StateDeadLetter, "retry schedule failed"); err != nil {
					return
				}
				_ = s.finishTerminal(ctx, rec.ID, model.StateDeadLetter, "retry schedule failed")
				return
			}
			return
		}
		if _, err := s.state.TransitionBundle(ctx, rec.ID, model.StateSimulating, model.StateDeadLetter, err.Error()); err != nil {
			return
		}
		rec, _, err = s.state.GetBundle(ctx, rec.ID)
		if err != nil {
			return
		}
		if err := s.emitTransition(ctx, rec, model.StateSimulating, model.StateDeadLetter, err.Error()); err != nil {
			return
		}
		if err := s.finishTerminal(ctx, rec.ID, model.StateDeadLetter, err.Error()); err != nil {
			return
		}
		return
	}

	if _, err := s.state.TransitionBundle(ctx, rec.ID, model.StateSimulating, model.StateSimulated, result.Reason); err != nil {
		return
	}
	rec, _, err = s.state.GetBundle(ctx, rec.ID)
	if err != nil {
		return
	}
	if err := s.emitTransition(ctx, rec, model.StateSimulating, model.StateSimulated, result.Reason); err != nil {
		return
	}

	if _, err := s.state.TransitionBundle(ctx, rec.ID, model.StateSimulated, model.StateScored, "scored"); err != nil {
		return
	}
	rec, _, err = s.state.GetBundle(ctx, rec.ID)
	if err != nil {
		return
	}
	if err := s.emitTransition(ctx, rec, model.StateSimulated, model.StateScored, "scored"); err != nil {
		return
	}

	action := model.StateRejected
	terminalReason := "score below threshold"
	if result.Success {
		action = model.StateForwarded
		terminalReason = "score accepted"
	}
	if _, err := s.state.TransitionBundle(ctx, rec.ID, model.StateScored, action, terminalReason); err != nil {
		return
	}
	rec, _, err = s.state.GetBundle(ctx, rec.ID)
	if err != nil {
		return
	}
	if err := s.emitTransition(ctx, rec, model.StateScored, action, terminalReason); err != nil {
		return
	}
	if _, err := s.state.UpdateResult(ctx, rec.ID, result.Score, result.ProfitEth, terminalReason); err != nil {
		return
	}
	if err := s.finishTerminal(ctx, rec.ID, action, terminalReason); err != nil {
		return
	}
}

func (s *Service) scheduleRetry(id string, retryCount int) error {
	delay := s.cfg.RetryBackoff * time.Duration(retryCount)
	return s.state.ScheduleRetry(context.Background(), id, time.Now().UTC().Add(delay))
}

func (s *Service) emitTransition(ctx context.Context, rec model.BundleRecord, from, to model.BundleState, reason string) error {
	ev := model.EventRecord{
		Time:       time.Now().UTC(),
		BundleID:   rec.ID,
		BundleHash: rec.BundleHash,
		From:       from,
		To:         to,
		Reason:     reason,
		Version:    rec.Version,
		Sequence:   rec.Sequence,
		ClientID:   rec.ClientID,
		RegionID:   rec.RegionID,
	}
	if err := s.state.AppendEvent(ctx, ev); err != nil {
		return err
	}
	if err := s.wal.Append(ctx, "event", ev); err != nil {
		return err
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return s.broker.Publish(ctx, broker.Message{
		Topic:     s.cfg.BrokerTopic,
		Key:       rec.ID,
		Sequence:  ev.Sequence,
		Timestamp: ev.Time,
		Headers:   map[string]string{"kind": "event"},
		Payload:   body,
	})
}

func (s *Service) finishTerminal(ctx context.Context, id string, terminal model.BundleState, reason string) error {
	rec, ok, err := s.state.GetBundle(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("bundle not found")
	}
	_, err = s.state.TransitionBundle(ctx, id, terminal, model.StatePersisted, reason)
	if err != nil {
		return err
	}
	rec, _, err = s.state.GetBundle(ctx, id)
	if err != nil {
		return err
	}
	if err := s.emitTransition(ctx, rec, terminal, model.StatePersisted, "persisted"); err != nil {
		return err
	}
	_, err = s.state.TransitionBundle(ctx, id, model.StatePersisted, model.StateCompleted, "completed")
	if err != nil {
		return err
	}
	rec, _, err = s.state.GetBundle(ctx, id)
	if err != nil {
		return err
	}
	if err := s.emitTransition(ctx, rec, model.StatePersisted, model.StateCompleted, "completed"); err != nil {
		return err
	}

	events, err := s.state.ListEvents(ctx, id, s.cfg.HistoryLimit)
	if err != nil {
		return err
	}
	leaves := make([][]byte, 0, len(events))
	for _, ev := range events {
		body, _ := json.Marshal(ev)
		leaves = append(leaves, body)
	}
	root := commitment.Root(leaves...)
	cp := model.CheckpointRecord{
		BatchID:    checkpointID(id, rec.Sequence),
		BundleID:   id,
		Root:       hex.EncodeToString(root[:]),
		EventCount: len(events),
		RegionID:   rec.RegionID,
		SignedBy:   "relay",
		Signature:  hex.EncodeToString(signature(root, rec)),
		Time:       time.Now().UTC(),
		Version:    rec.Version,
	}
	if err := s.state.PutCheckpoint(ctx, cp); err != nil {
		return err
	}
	if err := s.wal.Append(ctx, "checkpoint", cp); err != nil {
		return err
	}
	body, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	if err := s.broker.Publish(ctx, broker.Message{
		Topic:     s.cfg.BrokerTopic + ".checkpoint",
		Key:       id,
		Sequence:  uint64(cp.Version),
		Timestamp: cp.Time,
		Headers:   map[string]string{"kind": "checkpoint"},
		Payload:   body,
	}); err != nil {
		return err
	}
	if err := s.state.DeleteEvents(ctx, id); err != nil {
		return err
	}
	if _, err := s.state.ReleaseInflight(ctx, rec.ClientID); err != nil {
		return err
	}
	return nil
}

func retryable(err error) bool {
	type retryable interface{ Retryable() bool }
	var r retryable
	if errors.As(err, &r) {
		return r.Retryable()
	}
	return strings.Contains(err.Error(), "transient")
}

func bundleHash(p model.BundleRequest) string {
	h := sha256.New()
	for _, tx := range p.Txs {
		h.Write([]byte(tx))
	}
	h.Write([]byte(p.BlockNumber))
	h.Write([]byte(fmt.Sprintf("%d:%d", p.MinTimestamp, p.MaxTimestamp)))
	if p.Replacement != nil {
		h.Write([]byte(*p.Replacement))
	}
	return "0x" + hex.EncodeToString(h.Sum(nil)[:16])
}

func bundleID(hash string, reqID int64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", hash, reqID)))
	return "0x" + hex.EncodeToString(h[:12])
}

func checkpointID(bundleID string, version uint64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", bundleID, version)))
	return "cp_" + hex.EncodeToString(h[:12])
}

func signature(root [32]byte, rec model.BundleRecord) []byte {
	h := sha256.New()
	h.Write(root[:])
	h.Write([]byte(rec.ID))
	h.Write([]byte(rec.RegionID))
	return h.Sum(nil)
}
