package relay

import (
	"context"
	"time"

	"mevrelayv2/internal/model"
	"mevrelayv2/internal/telemetry"
)

// HealthState summarizes operational posture.
type HealthState string

const (
	HealthStateHealthy  HealthState = "healthy"
	HealthStateDegraded HealthState = "degraded"
	HealthStateDraining HealthState = "draining"
	HealthStateUnsafe   HealthState = "unsafe"
)

// HealthReport is the routing and diagnostics view of the relay.
type HealthReport struct {
	State            HealthState `json:"state"`
	Ready            bool        `json:"ready"`
	Reasons          []string    `json:"reasons,omitempty"`
	QueueDepth       int         `json:"queueDepth"`
	QueueCap         int         `json:"queueCap"`
	QueueOldestAgeMS int64       `json:"queueOldestAgeMs"`
	QueueNetValue    float64     `json:"queueNetValue"`
	QueueStaleCount  int         `json:"queueStaleCount"`
	RetryPending     int         `json:"retryPending"`
	RegionID         string      `json:"regionId"`
}

// AssessHealth checks the relay's runtime posture.
func (s *Service) AssessHealth(ctx context.Context) HealthReport {
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
		if (!item.DeadlineAt.IsZero() && now.After(item.DeadlineAt)) || (item.EnqueuedAt.IsZero() == false && now.Sub(item.EnqueuedAt) > s.cfg.MaxQueueAge) {
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

	report := HealthReport{
		State:            HealthStateHealthy,
		Ready:            true,
		QueueDepth:       s.queue.Len(),
		QueueCap:         s.queue.Cap(),
		QueueOldestAgeMS: oldest.Milliseconds(),
		QueueNetValue:    queueNet,
		QueueStaleCount:  staleCount,
		RetryPending:     retryPending,
		RegionID:         s.cfg.RegionID,
	}
	if s.draining.Load() && report.State != HealthStateUnsafe {
		report.State = HealthStateDraining
		report.Ready = false
		report.Reasons = append(report.Reasons, "draining")
	}

	if err := s.wal.Health(); err != nil {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "wal unhealthy")
	}
	s.metrics.ObserveWALLatency(time.Since(now))
	now = time.Now().UTC()
	if err := s.state.Health(ctx); err != nil {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "state unhealthy")
	}
	s.metrics.ObserveStateLatency(time.Since(now))
	now = time.Now().UTC()
	if err := s.backend.Ping(ctx); err != nil {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "backend unhealthy")
	}
	s.metrics.ObserveBackendLatency(time.Since(now))
	now = time.Now().UTC()
	if err := s.broker.Ping(ctx); err != nil {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "broker unhealthy")
	}
	s.metrics.ObserveBrokerLatency(time.Since(now))
	if report.QueueCap > 0 && report.QueueDepth*100/report.QueueCap >= 80 && report.State == HealthStateHealthy {
		report.State = HealthStateDegraded
		report.Reasons = append(report.Reasons, "queue pressure")
	}
	if report.QueueCap > 0 && report.QueueOldestAgeMS > int64(s.cfg.MaxQueueAge/time.Millisecond) {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "queue age exceeded")
	}
	if report.QueueDepth > 0 && report.QueueNetValue <= 0 && report.State == HealthStateHealthy {
		report.State = HealthStateDegraded
		report.Reasons = append(report.Reasons, "low queue value")
	}
	if report.RetryPending > 0 && report.State == HealthStateHealthy {
		report.State = HealthStateDegraded
		report.Reasons = append(report.Reasons, "retry debt")
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
	s.metrics.SetQueue(uint64(report.QueueDepth), uint64(report.QueueCap), uint64(report.QueueOldestAgeMS), report.QueueNetValue, retryDebt)
	return report
}
