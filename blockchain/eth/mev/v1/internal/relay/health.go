package relay

import (
	"context"
	"fmt"
)

type HealthState string

const (
	HealthStateHealthy  HealthState = "healthy"
	HealthStateDegraded HealthState = "degraded"
	HealthStateUnsafe   HealthState = "unsafe"
)

// HealthReport summarizes whether the relay can safely accept work.
type HealthReport struct {
	State          HealthState `json:"state"`
	Ready          bool        `json:"ready"`
	StoreHealthy   bool        `json:"store_healthy"`
	BackendHealthy bool        `json:"backend_healthy"`
	QueueDepth     int         `json:"queue_depth"`
	QueueCapacity  int         `json:"queue_capacity"`
	Reasons        []string    `json:"reasons,omitempty"`
}

// AssessHealth classifies the relay as healthy, degraded, or unsafe.
func (s *Service) AssessHealth(ctx context.Context) HealthReport {
	report := HealthReport{
		State:          HealthStateHealthy,
		Ready:          true,
		StoreHealthy:   true,
		BackendHealthy: true,
		QueueDepth:     len(s.queue),
		QueueCapacity:  cap(s.queue),
	}

	if err := s.store.Health(ctx); err != nil {
		report.StoreHealthy = false
		report.Ready = false
		report.State = HealthStateUnsafe
		report.Reasons = append(report.Reasons, fmt.Sprintf("store: %v", err))
	}
	if err := s.backend.Ping(ctx); err != nil {
		report.BackendHealthy = false
		report.Ready = false
		report.State = HealthStateUnsafe
		report.Reasons = append(report.Reasons, fmt.Sprintf("backend: %v", err))
	}

	if report.QueueCapacity > 0 {
		fill := float64(report.QueueDepth) / float64(report.QueueCapacity)
		switch {
		case report.QueueDepth >= report.QueueCapacity:
			report.Ready = false
			report.State = HealthStateUnsafe
			report.Reasons = append(report.Reasons, "queue saturated")
		case fill >= 0.8 && report.State == HealthStateHealthy:
			report.State = HealthStateDegraded
			report.Reasons = append(report.Reasons, "queue pressure")
		}
	}

	return report
}
