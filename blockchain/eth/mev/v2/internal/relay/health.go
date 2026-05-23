package relay

import (
	"context"
	"time"

	"mevrelayv2/internal/model"
)

// HealthState summarizes operational posture.
type HealthState string

const (
	HealthStateHealthy  HealthState = "healthy"
	HealthStateDegraded HealthState = "degraded"
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

	if err := s.wal.Health(); err != nil {
		report.State = HealthStateUnsafe
		report.Ready = false
		report.Reasons = append(report.Reasons, "wal unhealthy")
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
	return report
}
