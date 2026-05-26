package relay

import (
	"context"
	"time"

	"mevrelayv3/internal/model"
)

func (s *Service) settleBundle(ctx context.Context, id string, from, terminal model.BundleState, reason string) (model.BundleRecord, error) {
	rec, err := s.state.TransitionBundle(ctx, id, from, terminal, reason)
	if err != nil {
		return model.BundleRecord{}, err
	}
	if err := s.emitTransition(ctx, rec, from, terminal, reason); err != nil {
		return model.BundleRecord{}, err
	}
	rec, err = s.finishTerminal(ctx, rec, terminal, reason)
	if err != nil {
		return model.BundleRecord{}, err
	}
	return rec, err
}

func (s *Service) rejectBundle(ctx context.Context, id string, from model.BundleState, reason string) (model.BundleRecord, error) {
	return s.settleBundle(ctx, id, from, model.StateRejected, reason)
}

func (s *Service) deadLetterBundle(ctx context.Context, id string, from model.BundleState, reason string) (model.BundleRecord, error) {
	return s.settleBundle(ctx, id, from, model.StateDeadLetter, reason)
}

func (s *Service) scheduleRetry(ctx context.Context, id string, retryCount int) error {
	delay := s.policy.RetryBackoff() * time.Duration(retryCount)
	return s.state.ScheduleRetry(ctx, id, nowUTC().Add(delay))
}
