package relay

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"mevrelayv3/internal/model"
	"mevrelayv3/internal/telemetry"
)

type HealthState string

const (
	HealthStateHealthy  HealthState = "healthy"
	HealthStateDegraded HealthState = "degraded"
	HealthStateDraining HealthState = "draining"
	HealthStateUnsafe   HealthState = "unsafe"
)

type HealthReport struct {
	State            HealthState `json:"state"`
	Ready            bool        `json:"ready"`
	Reasons          []string    `json:"reasons,omitempty"`
	PolicyRevision   uint64      `json:"policyRevision"`
	PolicyPressure   float64     `json:"policyPressure"`
	PolicyConfidence float64     `json:"policyConfidence"`
	PolicyFloor      float64     `json:"policyFloor"`
	PolicyMinNet     float64     `json:"policyMinNetValue"`
	PolicyMinSlackMS int64       `json:"policyMinDeadlineSlackMs"`
	PolicyMaxAgeMS   int64       `json:"policyMaxQueueAgeMs"`
	PolicyBackoffMS  int64       `json:"policyRetryBackoffMs"`
	RecoveryState    string      `json:"recoveryState"`
	RecoveryReason   string      `json:"recoveryReason,omitempty"`
	RolloutState     string      `json:"rolloutState"`
	RolloutReason    string      `json:"rolloutReason,omitempty"`
	RolloutVersion   string      `json:"rolloutVersion,omitempty"`
	QueueDepth       int         `json:"queueDepth"`
	QueueCap         int         `json:"queueCap"`
	QueueOldestAgeMS int64       `json:"queueOldestAgeMs"`
	QueueNetValue    float64     `json:"queueNetValue"`
	QueueStaleCount  int         `json:"queueStaleCount"`
	RetryPending     int         `json:"retryPending"`
	AuthorityShard   string      `json:"authorityShard"`
	AuthorityEpoch   uint64      `json:"authorityEpoch"`
	AuthorityFresh   bool        `json:"authorityFresh"`
	RegionID         string      `json:"regionId"`
}

func (s *Service) AssessHealth(ctx context.Context) HealthReport {
	ctx, span := s.startSpan(ctx, "relay.health",
		attribute.String("shard.id", s.cfg.ShardID),
		attribute.String("region.id", s.cfg.RegionID),
	)
	defer span.End()
	now := time.Now().UTC()
	items := s.queue.Snapshot()
	oldest := s.queue.OldestAge(now)
	queueNet := 0.0
	staleCount := 0
	retryDebt := 0.0
	for _, item := range items {
		net := item.ExpectedValue - item.ExpectedCost
		if net > 0 {
			queueNet += net
		}
		if (!item.DeadlineAt.IsZero() && now.After(item.DeadlineAt)) || (!item.EnqueuedAt.IsZero() && now.Sub(item.EnqueuedAt) > s.cfg.MaxQueueAge) {
			staleCount++
		}
	}
	retryPending := 0
	if recs, err := s.state.ListBundles(ctx, s.cfg.HistoryLimit); err == nil {
		for _, rec := range recs {
			if rec.State == model.StateRetryPending {
				retryPending++
				weight := 1.0 + float64(rec.RetryCount)
				if !rec.UpdatedAt.IsZero() && s.cfg.MaxQueueAge > 0 {
					age := now.Sub(rec.UpdatedAt)
					if age > 0 {
						weight += float64(age) / float64(s.cfg.MaxQueueAge)
					}
				}
				retryDebt += weight
			}
		}
	}
	auth := s.currentAuthority()
	policy := s.policy.Snapshot()
	recovery := s.recovery.Snapshot()
	rollout := s.rollout.Snapshot()
	report := HealthReport{
		State:            HealthStateHealthy,
		Ready:            true,
		PolicyRevision:   policy.Revision,
		PolicyPressure:   policy.Pressure,
		PolicyConfidence: policy.Confidence,
		PolicyFloor:      policy.ConfidenceFloor,
		PolicyMinNet:     policy.MinNetValue,
		PolicyMinSlackMS: int64(policy.MinDeadlineSlack / time.Millisecond),
		PolicyMaxAgeMS:   int64(policy.MaxQueueAge / time.Millisecond),
		PolicyBackoffMS:  int64(policy.RetryBackoff / time.Millisecond),
		RecoveryState:    string(recovery.Stage),
		RecoveryReason:   recovery.LastReason,
		RolloutState:     string(rollout.Stage),
		RolloutReason:    rollout.Reason,
		RolloutVersion:   rollout.Version,
		QueueDepth:       s.queue.Len(),
		QueueCap:         s.queue.Cap(),
		QueueOldestAgeMS: oldest.Milliseconds(),
		QueueNetValue:    queueNet,
		QueueStaleCount:  staleCount,
		RetryPending:     retryPending,
		AuthorityShard:   auth.ShardID,
		AuthorityEpoch:   auth.Epoch,
		AuthorityFresh:   auth.Valid(now),
		RegionID:         s.cfg.RegionID,
	}
	if s.draining.Load() {
		report.State = HealthStateDraining
		report.Ready = false
		report.Reasons = append(report.Reasons, "draining")
	}
	if !s.rollout.Ready() && report.State == HealthStateHealthy {
		report.State = HealthStateDegraded
		report.Reasons = append(report.Reasons, "rollout in progress")
	}
	if !s.recovery.Ready() {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "recovery not ready")
	}
	if err := s.wal.Health(); err != nil {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "wal unhealthy")
	}
	if err := s.checkpts.Health(ctx); err != nil {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "checkpoint store unhealthy")
	}
	if err := s.policyStore.Health(ctx); err != nil {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "policy store unhealthy")
	}
	if err := s.state.Health(ctx); err != nil {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "state unhealthy")
	}
	if err := s.backend.Ping(ctx); err != nil {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "backend unhealthy")
	}
	if err := s.broker.Ping(ctx); err != nil {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "broker unhealthy")
	}
	if !report.AuthorityFresh {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "authority stale")
	}
	if report.QueueCap > 0 && report.QueueDepth*100/report.QueueCap >= policy.QueuePressurePct && report.State == HealthStateHealthy {
		report.State = HealthStateDegraded
		report.Reasons = append(report.Reasons, "queue pressure")
	}
	if report.QueueCap > 0 && report.QueueOldestAgeMS > int64(policy.MaxQueueAge/time.Millisecond) {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "queue age exceeded")
	}
	if report.QueueDepth > 0 && report.QueueNetValue <= policy.MinNetValue && report.State == HealthStateHealthy {
		report.State = HealthStateDegraded
		report.Reasons = append(report.Reasons, "low queue value")
	}
	if report.RetryPending > 0 && report.State == HealthStateHealthy {
		report.State = HealthStateDegraded
		report.Reasons = append(report.Reasons, "retry debt")
	}
	if report.PolicyConfidence < report.PolicyFloor {
		report.Ready = false
		if report.PolicyConfidence < report.PolicyFloor*0.5 {
			report.State = HealthStateUnsafe
			report.Reasons = append(report.Reasons, "policy confidence critically low")
		} else if report.State == HealthStateHealthy {
			report.State = HealthStateDegraded
			report.Reasons = append(report.Reasons, "policy confidence low")
		}
	}
	if report.QueueCap > 0 && report.QueueDepth >= report.QueueCap {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "queue full")
	}
	if report.QueueStaleCount > 0 {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "stale work present")
	}
	switch report.State {
	case HealthStateHealthy:
		s.metrics.SetHealthState(telemetry.HealthHealthy)
	case HealthStateDegraded:
		s.metrics.SetHealthState(telemetry.HealthDegraded)
	case HealthStateDraining:
		s.metrics.SetHealthState(telemetry.HealthDraining)
	default:
		s.metrics.SetHealthState(telemetry.HealthUnsafe)
	}
	s.metrics.SetRecoveryState(report.RecoveryState)
	s.metrics.SetRolloutState(report.RolloutState)
	s.metrics.SetQueue(uint64(report.QueueDepth), uint64(report.QueueCap), uint64(report.QueueOldestAgeMS), report.QueueNetValue, retryDebt)
	s.metrics.SetPolicy(report.PolicyRevision, report.PolicyPressure, report.PolicyConfidence)
	span.SetAttributes(
		attribute.String("health.state", string(report.State)),
		attribute.Bool("health.ready", report.Ready),
		attribute.Int("queue.depth", report.QueueDepth),
		attribute.Int("queue.cap", report.QueueCap),
		attribute.Bool("authority.fresh", report.AuthorityFresh),
		attribute.String("recovery.state", report.RecoveryState),
		attribute.String("rollout.state", report.RolloutState),
	)
	return report
}
