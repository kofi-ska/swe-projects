package relay

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"mevrelayv3/internal/model"
	"mevrelayv3/internal/scheduler"
)

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
	interval := s.policy.RetryBackoff() / 2
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

func (s *Service) authorityLoop(ctx context.Context) {
	defer s.wg.Done()
	interval := s.cfg.LeaseRenewInterval
	if interval <= 0 {
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
			s.renewAuthority(ctx)
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
	ctx, span := s.startSpan(ctx, "relay.retry",
		attribute.String("bundle.id", id),
		attribute.String("shard.id", s.cfg.ShardID),
	)
	defer span.End()
	rec, ok, err := s.state.GetBundle(ctx, id)
	if err != nil || !ok {
		endSpan(span, err)
		return
	}
	if rec.ShardID != s.cfg.ShardID {
		s.metrics.IncWrongShard()
		endSpan(span, ErrWrongShard)
		return
	}
	if rec.State != model.StateRetryPending {
		return
	}
	if rec.RetryCount >= s.cfg.MaxRetries {
		_, _ = s.deadLetterBundle(ctx, id, model.StateRetryPending, "retry exhausted")
		return
	}
	updated, err := s.state.TransitionBundle(ctx, id, model.StateRetryPending, model.StateQueued, "retry queued")
	if err != nil {
		return
	}
	rec = updated
	if err := s.emitTransition(ctx, rec, model.StateRetryPending, model.StateQueued, "retry queued"); err != nil {
		return
	}
	now := time.Now().UTC()
	if !rec.DeadlineAt.IsZero() && now.After(rec.DeadlineAt) {
		_, _ = s.deadLetterBundle(ctx, id, model.StateQueued, "retry expired")
		return
	}
	if rec.ExpectedValue-rec.ExpectedCost < s.policy.MinNetValue() {
		_, _ = s.deadLetterBundle(ctx, id, model.StateQueued, "retry uneconomic")
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
		s.metrics.IncQueueOverflow()
		_, _ = s.deadLetterBundle(ctx, id, model.StateQueued, "retry queue overflow")
		return
	}
	if evicted != nil {
		_, _ = s.rejectBundle(ctx, evicted.ID, model.StateQueued, "priority eviction")
	}
}

func (s *Service) process(ctx context.Context, item scheduler.Item) {
	ctx, span := s.startSpan(ctx, "relay.process",
		attribute.String("bundle.id", item.ID),
		attribute.String("shard.id", s.cfg.ShardID),
	)
	defer span.End()
	if !s.authorityValid() {
		s.metrics.IncStaleAuthority()
		endSpan(span, ErrStaleAuthority)
		return
	}
	rec, ok, err := s.state.GetBundle(ctx, item.ID)
	if err != nil || !ok {
		endSpan(span, err)
		return
	}
	if rec.ShardID != s.cfg.ShardID {
		s.metrics.IncWrongShard()
		endSpan(span, ErrWrongShard)
		return
	}
	now := time.Now().UTC()
	if !rec.DeadlineAt.IsZero() && now.After(rec.DeadlineAt) {
		_, _ = s.rejectBundle(ctx, rec.ID, model.StateQueued, "expired in queue")
		return
	}
	if !rec.QueuedAt.IsZero() && now.Sub(rec.QueuedAt) > s.policy.MaxQueueAge() {
		_, _ = s.rejectBundle(ctx, rec.ID, model.StateQueued, "queue age exceeded")
		return
	}
	if rec.ExpectedValue-rec.ExpectedCost < s.policy.MinNetValue() {
		_, _ = s.rejectBundle(ctx, rec.ID, model.StateQueued, "insufficient priority")
		return
	}
	if !rec.DeadlineAt.IsZero() && time.Duration(rec.ExpectedServiceMS)*time.Millisecond > time.Until(rec.DeadlineAt) {
		_, _ = s.rejectBundle(ctx, rec.ID, model.StateQueued, "insufficient slack")
		return
	}
	updated, err := s.state.TransitionBundle(ctx, rec.ID, model.StateQueued, model.StateSimulating, "picked")
	if err != nil {
		endSpan(span, err)
		return
	}
	rec = updated
	if err := s.emitTransition(ctx, rec, model.StateQueued, model.StateSimulating, "picked"); err != nil {
		endSpan(span, err)
		return
	}
	simCtx, cancel := context.WithTimeout(ctx, s.cfg.RequestTimeout)
	defer cancel()
	simCtx, simSpan := s.startSpan(simCtx, "relay.simulate",
		attribute.String("bundle.id", rec.ID),
		attribute.String("shard.id", s.cfg.ShardID),
	)
	result, err := s.backend.Simulate(simCtx, rec)
	endSpan(simSpan, err)
	simSpan.End()
	if err != nil {
		s.metrics.IncBackendError()
		if retryable(err) && rec.RetryCount+1 <= s.cfg.MaxRetries {
			rec, err = s.state.TransitionBundle(ctx, rec.ID, model.StateSimulating, model.StateRetryPending, "transient failure")
			if err != nil {
				endSpan(span, err)
				return
			}
			if err := s.emitTransition(ctx, rec, model.StateSimulating, model.StateRetryPending, "transient failure"); err != nil {
				endSpan(span, err)
				return
			}
			rec, err = s.state.UpdateRetryCount(ctx, rec.ID, rec.RetryCount+1)
			if err != nil {
				endSpan(span, err)
				return
			}
			s.metrics.IncRetryPending()
			s.metrics.IncRetryScheduled()
			if err := s.scheduleRetry(ctx, rec.ID, rec.RetryCount); err != nil {
				s.metrics.IncTerminalError()
				_, _ = s.deadLetterBundle(ctx, rec.ID, model.StateRetryPending, "retry schedule failed")
				endSpan(span, err)
				return
			}
			return
		}
		_, _ = s.deadLetterBundle(ctx, rec.ID, model.StateSimulating, err.Error())
		endSpan(span, err)
		return
	}
	rec, err = s.state.TransitionBundle(ctx, rec.ID, model.StateSimulating, model.StateSimulated, result.Reason)
	if err != nil {
		endSpan(span, err)
		return
	}
	if err := s.emitTransition(ctx, rec, model.StateSimulating, model.StateSimulated, result.Reason); err != nil {
		endSpan(span, err)
		return
	}
	rec, err = s.state.TransitionBundle(ctx, rec.ID, model.StateSimulated, model.StateScored, "scored")
	if err != nil {
		endSpan(span, err)
		return
	}
	if err := s.emitTransition(ctx, rec, model.StateSimulated, model.StateScored, "scored"); err != nil {
		endSpan(span, err)
		return
	}
	action := model.StateRejected
	terminalReason := "score below threshold"
	if result.Success {
		action = model.StateForwarded
		terminalReason = "score accepted"
	}
	rec, err = s.state.TransitionBundle(ctx, rec.ID, model.StateScored, action, terminalReason)
	if err != nil {
		endSpan(span, err)
		return
	}
	if err := s.emitTransition(ctx, rec, model.StateScored, action, terminalReason); err != nil {
		endSpan(span, err)
		return
	}
	rec, err = s.state.UpdateResult(ctx, rec.ID, result.Score, result.ProfitEth, terminalReason)
	if err != nil {
		endSpan(span, err)
		return
	}
	if _, err := s.finishTerminal(ctx, rec, action, terminalReason); err != nil {
		endSpan(span, err)
		return
	}
}

func retryable(err error) bool {
	type retryableErr interface{ Retryable() bool }
	var r retryableErr
	if errors.As(err, &r) {
		return r.Retryable()
	}
	return strings.Contains(err.Error(), "transient")
}
