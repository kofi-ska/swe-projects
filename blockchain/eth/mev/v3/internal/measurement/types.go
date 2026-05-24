package measurement

import (
	"time"

	"mevrelayv3/internal/relay"
	"mevrelayv3/internal/telemetry"
)

type Mode string

const (
	ModeSteady  Mode = "steady"
	ModeBurst   Mode = "burst"
	ModeFailure Mode = "failure"
	ModeReplay  Mode = "replay"
)

type Scenario struct {
	Name            string
	Mode            Mode
	Duration        time.Duration
	Warmup          time.Duration
	Cooldown        time.Duration
	Concurrency     int
	RatePerSecond   int
	Requests        int
	TxCount         int
	TxBytes         int
	DuplicateEvery  int
	ClientID        string
	RegionID        string
	ShardID         string
	Authorization   string
	CompareBaseline string
}

type PhaseStats struct {
	Count int64         `json:"count"`
	Mean  time.Duration `json:"mean"`
	P95   time.Duration `json:"p95"`
	P99   time.Duration `json:"p99"`
	Max   time.Duration `json:"max"`
}

type Result struct {
	Scenario      string                `json:"scenario"`
	Mode          Mode                  `json:"mode"`
	StartedAt     time.Time             `json:"startedAt"`
	FinishedAt    time.Time             `json:"finishedAt"`
	Duration      time.Duration         `json:"duration"`
	Requests      int                   `json:"requests"`
	Successes     int                   `json:"successes"`
	Failures      int                   `json:"failures"`
	Accepted      int                   `json:"accepted"`
	Rejected      int                   `json:"rejected"`
	MeanLatency   time.Duration         `json:"meanLatency"`
	P95Latency    time.Duration         `json:"p95Latency"`
	P99Latency    time.Duration         `json:"p99Latency"`
	MaxLatency    time.Duration         `json:"maxLatency"`
	HealthBefore  relay.HealthReport    `json:"healthBefore"`
	HealthAfter   relay.HealthReport    `json:"healthAfter"`
	MetricsBefore telemetry.Snapshot    `json:"metricsBefore"`
	MetricsAfter  telemetry.Snapshot    `json:"metricsAfter"`
	Regression    bool                  `json:"regression"`
	RegressionWhy []string              `json:"regressionWhy,omitempty"`
	PhaseStats    map[string]PhaseStats `json:"phaseStats,omitempty"`
}

type Report struct {
	GeneratedAt time.Time `json:"generatedAt"`
	Results     []Result  `json:"results"`
}

type Baseline struct {
	Name   string `json:"name"`
	Result Result `json:"result"`
}
