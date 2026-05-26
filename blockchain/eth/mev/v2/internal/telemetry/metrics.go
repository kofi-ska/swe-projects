package telemetry

import (
	"fmt"
	"math"
	"strings"
	"sync/atomic"
	"time"
)

const (
	HealthUnknown uint32 = iota
	HealthHealthy
	HealthDegraded
	HealthDraining
	HealthUnsafe
)

// Metrics captures relay runtime counters and gauges for alerts and ops.
type Metrics struct {
	submitted      atomic.Uint64
	accepted       atomic.Uint64
	rejected       atomic.Uint64
	forwarded      atomic.Uint64
	deadLetters    atomic.Uint64
	retryPending   atomic.Uint64
	retryScheduled atomic.Uint64
	duplicates     atomic.Uint64
	queueOverflow  atomic.Uint64
	inflightLimit  atomic.Uint64
	backendErrors  atomic.Uint64
	stateErrors    atomic.Uint64
	walErrors      atomic.Uint64
	brokerErrors   atomic.Uint64
	terminalErrors atomic.Uint64

	queueDepth       atomic.Uint64
	queueCap         atomic.Uint64
	queueOldestAgeMS atomic.Uint64
	queueNetValue    atomic.Uint64
	retryDebt        atomic.Uint64
	healthState      atomic.Uint32

	backendLatencyMS atomic.Uint64
	stateLatencyMS   atomic.Uint64
	brokerLatencyMS  atomic.Uint64
	walLatencyMS     atomic.Uint64
}

// Snapshot is a stable view suitable for alerting and metrics export.
type Snapshot struct {
	Submitted        uint64
	Accepted         uint64
	Rejected         uint64
	Forwarded        uint64
	DeadLetters      uint64
	RetryPending     uint64
	RetryScheduled   uint64
	Duplicates       uint64
	QueueOverflow    uint64
	InflightLimit    uint64
	BackendErrors    uint64
	StateErrors      uint64
	WALErrors        uint64
	BrokerErrors     uint64
	TerminalErrors   uint64
	QueueDepth       uint64
	QueueCap         uint64
	QueueOldestAgeMS uint64
	QueueNetValue    float64
	RetryDebt        float64
	HealthStateCode  uint32
	HealthState      string
	BackendLatencyMS uint64
	StateLatencyMS   uint64
	BrokerLatencyMS  uint64
	WALLatencyMS     uint64
}

// New returns a ready-to-use metrics collector.
func New() *Metrics { return &Metrics{} }

func (m *Metrics) IncSubmitted()      { m.submitted.Add(1) }
func (m *Metrics) IncAccepted()       { m.accepted.Add(1) }
func (m *Metrics) IncRejected()       { m.rejected.Add(1) }
func (m *Metrics) IncForwarded()      { m.forwarded.Add(1) }
func (m *Metrics) IncDeadLetter()     { m.deadLetters.Add(1) }
func (m *Metrics) IncRetryPending()   { m.retryPending.Add(1) }
func (m *Metrics) IncRetryScheduled() { m.retryScheduled.Add(1) }
func (m *Metrics) IncDuplicate()      { m.duplicates.Add(1) }
func (m *Metrics) IncQueueOverflow()  { m.queueOverflow.Add(1) }
func (m *Metrics) IncInflightLimit()  { m.inflightLimit.Add(1) }
func (m *Metrics) IncBackendError()   { m.backendErrors.Add(1) }
func (m *Metrics) IncStateError()     { m.stateErrors.Add(1) }
func (m *Metrics) IncWALError()       { m.walErrors.Add(1) }
func (m *Metrics) IncBrokerError()    { m.brokerErrors.Add(1) }
func (m *Metrics) IncTerminalError()  { m.terminalErrors.Add(1) }

func (m *Metrics) SetHealthState(state uint32) { m.healthState.Store(state) }
func (m *Metrics) SetQueue(depth, cap uint64, oldestAgeMS uint64, netValue float64, retryDebt float64) {
	m.queueDepth.Store(depth)
	m.queueCap.Store(cap)
	m.queueOldestAgeMS.Store(oldestAgeMS)
	m.queueNetValue.Store(math.Float64bits(netValue))
	m.retryDebt.Store(math.Float64bits(retryDebt))
}

func (m *Metrics) ObserveBackendLatency(d time.Duration) {
	m.backendLatencyMS.Store(uint64(d.Milliseconds()))
}
func (m *Metrics) ObserveStateLatency(d time.Duration) {
	m.stateLatencyMS.Store(uint64(d.Milliseconds()))
}
func (m *Metrics) ObserveBrokerLatency(d time.Duration) {
	m.brokerLatencyMS.Store(uint64(d.Milliseconds()))
}
func (m *Metrics) ObserveWALLatency(d time.Duration) { m.walLatencyMS.Store(uint64(d.Milliseconds())) }

func (m *Metrics) Snapshot() Snapshot {
	state := "unknown"
	switch m.healthState.Load() {
	case HealthHealthy:
		state = "healthy"
	case HealthDegraded:
		state = "degraded"
	case HealthDraining:
		state = "draining"
	case HealthUnsafe:
		state = "unsafe"
	}
	return Snapshot{
		Submitted:        m.submitted.Load(),
		Accepted:         m.accepted.Load(),
		Rejected:         m.rejected.Load(),
		Forwarded:        m.forwarded.Load(),
		DeadLetters:      m.deadLetters.Load(),
		RetryPending:     m.retryPending.Load(),
		RetryScheduled:   m.retryScheduled.Load(),
		Duplicates:       m.duplicates.Load(),
		QueueOverflow:    m.queueOverflow.Load(),
		InflightLimit:    m.inflightLimit.Load(),
		BackendErrors:    m.backendErrors.Load(),
		StateErrors:      m.stateErrors.Load(),
		WALErrors:        m.walErrors.Load(),
		BrokerErrors:     m.brokerErrors.Load(),
		TerminalErrors:   m.terminalErrors.Load(),
		QueueDepth:       m.queueDepth.Load(),
		QueueCap:         m.queueCap.Load(),
		QueueOldestAgeMS: m.queueOldestAgeMS.Load(),
		QueueNetValue:    math.Float64frombits(m.queueNetValue.Load()),
		RetryDebt:        math.Float64frombits(m.retryDebt.Load()),
		HealthStateCode:  m.healthState.Load(),
		HealthState:      state,
		BackendLatencyMS: m.backendLatencyMS.Load(),
		StateLatencyMS:   m.stateLatencyMS.Load(),
		BrokerLatencyMS:  m.brokerLatencyMS.Load(),
		WALLatencyMS:     m.walLatencyMS.Load(),
	}
}

// RenderPrometheus emits a stable Prometheus text payload.
func (m *Metrics) RenderPrometheus() string {
	s := m.Snapshot()
	var b strings.Builder
	appendCounter := func(name string, v uint64) {
		fmt.Fprintf(&b, "%s %d\n", name, v)
	}
	appendGauge := func(name string, v any) {
		fmt.Fprintf(&b, "%s %v\n", name, v)
	}
	appendCounter("mevrelay_submitted_total", s.Submitted)
	appendCounter("mevrelay_accepted_total", s.Accepted)
	appendCounter("mevrelay_rejected_total", s.Rejected)
	appendCounter("mevrelay_forwarded_total", s.Forwarded)
	appendCounter("mevrelay_dead_letters_total", s.DeadLetters)
	appendCounter("mevrelay_retry_pending_total", s.RetryPending)
	appendCounter("mevrelay_retry_scheduled_total", s.RetryScheduled)
	appendCounter("mevrelay_duplicates_total", s.Duplicates)
	appendCounter("mevrelay_queue_overflow_total", s.QueueOverflow)
	appendCounter("mevrelay_inflight_limit_total", s.InflightLimit)
	appendCounter("mevrelay_backend_errors_total", s.BackendErrors)
	appendCounter("mevrelay_state_errors_total", s.StateErrors)
	appendCounter("mevrelay_wal_errors_total", s.WALErrors)
	appendCounter("mevrelay_broker_errors_total", s.BrokerErrors)
	appendCounter("mevrelay_terminal_errors_total", s.TerminalErrors)
	appendGauge("mevrelay_queue_depth", s.QueueDepth)
	appendGauge("mevrelay_queue_capacity", s.QueueCap)
	appendGauge("mevrelay_queue_oldest_age_ms", s.QueueOldestAgeMS)
	appendGauge("mevrelay_queue_net_value", s.QueueNetValue)
	appendGauge("mevrelay_retry_debt", s.RetryDebt)
	appendGauge("mevrelay_health_state", s.HealthStateCode)
	appendGauge("mevrelay_backend_latency_ms", s.BackendLatencyMS)
	appendGauge("mevrelay_state_latency_ms", s.StateLatencyMS)
	appendGauge("mevrelay_broker_latency_ms", s.BrokerLatencyMS)
	appendGauge("mevrelay_wal_latency_ms", s.WALLatencyMS)
	return b.String()
}
