package metrics

import "sync/atomic"

// Metrics holds relay counters used for state and failure reporting.
type Metrics struct {
	Received       atomic.Int64
	Queued         atomic.Int64
	Simulated      atomic.Int64
	Forwarded      atomic.Int64
	Rejected       atomic.Int64
	DeadLettered   atomic.Int64
	RetryPending   atomic.Int64
	SimulationFail atomic.Int64
	QueueRejects   atomic.Int64
}

// Snapshot returns the current metric values.
func (m *Metrics) Snapshot() map[string]int64 {
	return map[string]int64{
		"received":        m.Received.Load(),
		"queued":          m.Queued.Load(),
		"simulated":       m.Simulated.Load(),
		"forwarded":       m.Forwarded.Load(),
		"rejected":        m.Rejected.Load(),
		"dead_lettered":   m.DeadLettered.Load(),
		"retry_pending":   m.RetryPending.Load(),
		"simulation_fail": m.SimulationFail.Load(),
		"queue_rejects":   m.QueueRejects.Load(),
	}
}
