package relay

import (
	"math"
	"sync/atomic"
	"time"

	"mevrelayv3/internal/config"
	"mevrelayv3/internal/telemetry"
)

type PolicySnapshot struct {
	Revision         uint64
	Pressure         float64
	Confidence       float64
	MinNetValue      float64
	MinDeadlineSlack time.Duration
	MaxQueueAge      time.Duration
	RetryBackoff     time.Duration
	QueuePressurePct int
	ConfidenceFloor  float64
}

type ControlPolicy struct {
	baseMinNetValue      float64
	baseMinDeadlineSlack time.Duration
	baseMaxQueueAge      time.Duration
	baseRetryBackoff     time.Duration
	baseQueuePressurePct int
	baseConfidenceFloor  float64

	minNetValueBits    atomic.Uint64
	minSlackMS         atomic.Int64
	maxQueueAgeMS      atomic.Int64
	retryBackoffMS     atomic.Int64
	queuePressurePct   atomic.Int64
	confidenceFloorBit atomic.Uint64
	revision           atomic.Uint64
	pressureBits       atomic.Uint64
	confidenceBits     atomic.Uint64
}

func (p *ControlPolicy) ApplySnapshot(snap PolicySnapshot) {
	if snap.Revision > 0 {
		p.revision.Store(snap.Revision)
	}
	p.pressureBits.Store(math.Float64bits(clamp(snap.Pressure, 0, 1)))
	p.confidenceBits.Store(math.Float64bits(clamp(snap.Confidence, 0, 1)))
	if snap.MinNetValue >= 0 {
		p.minNetValueBits.Store(math.Float64bits(snap.MinNetValue))
	}
	if snap.MinDeadlineSlack > 0 {
		p.minSlackMS.Store(int64(snap.MinDeadlineSlack / time.Millisecond))
	}
	if snap.MaxQueueAge > 0 {
		p.maxQueueAgeMS.Store(int64(snap.MaxQueueAge / time.Millisecond))
	}
	if snap.RetryBackoff > 0 {
		p.retryBackoffMS.Store(int64(snap.RetryBackoff / time.Millisecond))
	}
	if snap.QueuePressurePct > 0 {
		p.queuePressurePct.Store(int64(snap.QueuePressurePct))
	}
	if snap.ConfidenceFloor > 0 {
		p.confidenceFloorBit.Store(math.Float64bits(snap.ConfidenceFloor))
	}
}

func NewControlPolicy(cfg config.Config) *ControlPolicy {
	p := &ControlPolicy{
		baseMinNetValue:      cfg.MinNetValue,
		baseMinDeadlineSlack: cfg.MinDeadlineSlack,
		baseMaxQueueAge:      cfg.MaxQueueAge,
		baseRetryBackoff:     cfg.RetryBackoff,
		baseQueuePressurePct: 80,
		baseConfidenceFloor:  0.55,
	}
	p.minNetValueBits.Store(math.Float64bits(cfg.MinNetValue))
	p.minSlackMS.Store(int64(cfg.MinDeadlineSlack / time.Millisecond))
	p.maxQueueAgeMS.Store(int64(cfg.MaxQueueAge / time.Millisecond))
	p.retryBackoffMS.Store(int64(cfg.RetryBackoff / time.Millisecond))
	p.queuePressurePct.Store(int64(p.baseQueuePressurePct))
	p.confidenceFloorBit.Store(math.Float64bits(p.baseConfidenceFloor))
	p.revision.Store(1)
	p.pressureBits.Store(math.Float64bits(0))
	p.confidenceBits.Store(math.Float64bits(1))
	return p
}

func (p *ControlPolicy) Snapshot() PolicySnapshot {
	return PolicySnapshot{
		Revision:         p.revision.Load(),
		Pressure:         math.Float64frombits(p.pressureBits.Load()),
		Confidence:       math.Float64frombits(p.confidenceBits.Load()),
		MinNetValue:      math.Float64frombits(p.minNetValueBits.Load()),
		MinDeadlineSlack: time.Duration(p.minSlackMS.Load()) * time.Millisecond,
		MaxQueueAge:      time.Duration(p.maxQueueAgeMS.Load()) * time.Millisecond,
		RetryBackoff:     time.Duration(p.retryBackoffMS.Load()) * time.Millisecond,
		QueuePressurePct: int(p.queuePressurePct.Load()),
		ConfidenceFloor:  math.Float64frombits(p.confidenceFloorBit.Load()),
	}
}

func (p *ControlPolicy) MinNetValue() float64 {
	return math.Float64frombits(p.minNetValueBits.Load())
}

func (p *ControlPolicy) MinDeadlineSlack() time.Duration {
	return time.Duration(p.minSlackMS.Load()) * time.Millisecond
}

func (p *ControlPolicy) MaxQueueAge() time.Duration {
	return time.Duration(p.maxQueueAgeMS.Load()) * time.Millisecond
}

func (p *ControlPolicy) RetryBackoff() time.Duration {
	return time.Duration(p.retryBackoffMS.Load()) * time.Millisecond
}

func (p *ControlPolicy) QueuePressurePct() int {
	return int(p.queuePressurePct.Load())
}

func (p *ControlPolicy) ConfidenceFloor() float64 {
	return math.Float64frombits(p.confidenceFloorBit.Load())
}

func (p *ControlPolicy) Confidence() float64 {
	return math.Float64frombits(p.confidenceBits.Load())
}

func (p *ControlPolicy) Adapt(cfg config.Config, snap telemetry.Snapshot, report HealthReport) PolicySnapshot {
	pressure := p.estimatePressure(cfg, snap, report)
	confidence := clamp(1-pressure, 0, 1)
	if !report.AuthorityFresh {
		pressure = clamp(pressure+0.25, 0, 1)
		confidence = 0
	}
	switch report.State {
	case HealthStateDegraded:
		pressure = clamp(pressure+0.1, 0, 1)
	case HealthStateUnsafe:
		pressure = clamp(pressure+0.25, 0, 1)
		confidence = math.Min(confidence, 0.2)
	}
	if report.QueueStaleCount > 0 {
		pressure = clamp(pressure+0.15, 0, 1)
	}
	if report.QueueDepth > 0 && report.QueueNetValue <= 0 {
		pressure = clamp(pressure+0.05, 0, 1)
	}

	targetMinNet := p.baseMinNetValue + pressure*(cfg.CostPerTx+cfg.CostPerMS*float64(cfg.RequestTimeout/time.Millisecond))
	targetSlack := p.baseMinDeadlineSlack + time.Duration(pressure*float64(cfg.RequestTimeout)/2)
	targetAge := time.Duration(float64(p.baseMaxQueueAge) * (1 - pressure*0.6))
	if targetAge < time.Second {
		targetAge = time.Second
	}
	targetBackoff := time.Duration(float64(p.baseRetryBackoff) * (1 + pressure))
	if targetBackoff < 100*time.Millisecond {
		targetBackoff = 100 * time.Millisecond
	}
	maxBackoff := p.baseRetryBackoff * 5
	if targetBackoff > maxBackoff {
		targetBackoff = maxBackoff
	}
	targetPressurePct := p.baseQueuePressurePct - int(math.Round(pressure*20))
	if targetPressurePct < 60 {
		targetPressurePct = 60
	}
	if targetPressurePct > p.baseQueuePressurePct {
		targetPressurePct = p.baseQueuePressurePct
	}
	targetFloor := p.baseConfidenceFloor + pressure*0.3
	if targetFloor > 0.95 {
		targetFloor = 0.95
	}
	if targetFloor < 0.3 {
		targetFloor = 0.3
	}

	minNet := smoothFloat(math.Float64frombits(p.minNetValueBits.Load()), targetMinNet, 0.35)
	minSlack := smoothDuration(time.Duration(p.minSlackMS.Load())*time.Millisecond, targetSlack, 0.35)
	maxAge := smoothDuration(time.Duration(p.maxQueueAgeMS.Load())*time.Millisecond, targetAge, 0.35)
	backoff := smoothDuration(time.Duration(p.retryBackoffMS.Load())*time.Millisecond, targetBackoff, 0.35)
	pressurePct := smoothInt(int(p.queuePressurePct.Load()), targetPressurePct, 0.35)
	floor := smoothFloat(math.Float64frombits(p.confidenceFloorBit.Load()), targetFloor, 0.35)

	changed := p.minNetValueBits.Load() != math.Float64bits(minNet) ||
		p.minSlackMS.Load() != int64(minSlack/time.Millisecond) ||
		p.maxQueueAgeMS.Load() != int64(maxAge/time.Millisecond) ||
		p.retryBackoffMS.Load() != int64(backoff/time.Millisecond) ||
		p.queuePressurePct.Load() != int64(pressurePct) ||
		p.confidenceFloorBit.Load() != math.Float64bits(floor)

	if changed {
		p.minNetValueBits.Store(math.Float64bits(minNet))
		p.minSlackMS.Store(int64(minSlack / time.Millisecond))
		p.maxQueueAgeMS.Store(int64(maxAge / time.Millisecond))
		p.retryBackoffMS.Store(int64(backoff / time.Millisecond))
		p.queuePressurePct.Store(int64(pressurePct))
		p.confidenceFloorBit.Store(math.Float64bits(floor))
		p.revision.Add(1)
	}
	p.pressureBits.Store(math.Float64bits(pressure))
	p.confidenceBits.Store(math.Float64bits(confidence))

	return p.Snapshot()
}

func (p *ControlPolicy) estimatePressure(cfg config.Config, snap telemetry.Snapshot, report HealthReport) float64 {
	pressure := 0.0
	if report.QueueCap > 0 {
		pressure += 0.35 * clamp(float64(report.QueueDepth)/float64(report.QueueCap), 0, 1)
	}
	if report.QueueOldestAgeMS > 0 && cfg.MaxQueueAge > 0 {
		pressure += 0.25 * clamp(float64(report.QueueOldestAgeMS)/float64(cfg.MaxQueueAge/time.Millisecond), 0, 2) / 2
	}
	if snap.RetryDebt > 0 {
		pressure += 0.15 * clamp(snap.RetryDebt/float64(max(1, cfg.HistoryLimit)), 0, 1)
	}
	maxLatency := maxUint64(snap.BackendLatencyMS, snap.StateLatencyMS, snap.BrokerLatencyMS, snap.WALLatencyMS, snap.CheckpointLatencyMS)
	if cfg.RequestTimeout > 0 {
		pressure += 0.15 * clamp(float64(maxLatency)/float64(cfg.RequestTimeout/time.Millisecond), 0, 2) / 2
	}
	if report.QueueNetValue <= 0 && report.QueueDepth > 0 {
		pressure += 0.05
	}
	if report.RetryPending > 0 {
		pressure += 0.05
	}
	if report.State == HealthStateDegraded {
		pressure += 0.05
	}
	if report.State == HealthStateUnsafe {
		pressure += 0.1
	}
	return clamp(pressure, 0, 1)
}

func smoothFloat(current, target, alpha float64) float64 {
	if alpha <= 0 {
		alpha = 0.35
	}
	if alpha > 1 {
		alpha = 1
	}
	return current + (target-current)*alpha
}

func smoothDuration(current, target time.Duration, alpha float64) time.Duration {
	cur := float64(current)
	tgt := float64(target)
	val := smoothFloat(cur, tgt, alpha)
	if val < 0 {
		val = 0
	}
	return time.Duration(val)
}

func smoothInt(current, target int, alpha float64) int {
	val := smoothFloat(float64(current), float64(target), alpha)
	return int(math.Round(val))
}

func maxUint64(v ...uint64) uint64 {
	var m uint64
	for _, x := range v {
		if x > m {
			m = x
		}
	}
	return m
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
