package measurement

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"mevrelayv3/internal/model"
)

type Runner struct {
	Target   Target
	ClientID string
	RegionID string
}

func (r Runner) Run(ctx context.Context, scenarios ...Scenario) (Report, error) {
	if r.Target == nil {
		return Report{}, fmt.Errorf("missing target")
	}
	report := Report{GeneratedAt: time.Now().UTC()}
	for _, scenario := range scenarios {
		res, err := r.runScenario(ctx, scenario)
		if err != nil {
			return Report{}, err
		}
		report.Results = append(report.Results, res)
	}
	return report, nil
}

func (r Runner) runScenario(ctx context.Context, scenario Scenario) (Result, error) {
	if scenario.Concurrency <= 0 {
		scenario.Concurrency = 1
	}
	if scenario.TxCount <= 0 {
		scenario.TxCount = 1
	}
	warmup := maxDuration(0, scenario.Warmup)
	duration := scenario.Duration
	if duration <= 0 {
		duration = 10 * time.Second
	}
	cooldown := maxDuration(0, scenario.Cooldown)
	window := warmup + duration + cooldown
	if scenario.Requests <= 0 {
		if scenario.RatePerSecond > 0 {
			scenario.Requests = int(math.Ceil(float64(window) * float64(scenario.RatePerSecond) / float64(time.Second)))
		} else {
			scenario.Requests = scenario.Concurrency * int(window/time.Second)
			if scenario.Requests <= 0 {
				scenario.Requests = scenario.Concurrency * 100
			}
		}
	}
	beforeHealth, _ := r.Target.Health(ctx)
	beforeMetrics, _ := r.Target.Metrics(ctx)
	start := time.Now().UTC()
	activeStart := start.Add(warmup)
	activeEnd := activeStart.Add(duration)
	var (
		mu       sync.Mutex
		samples  = make([]time.Duration, 0, scenario.Requests)
		success  int
		failures int
		accepted int
		rejected int
	)
	jobs := make(chan int, scenario.Concurrency)
	var wg sync.WaitGroup
	worker := func(idx int) {
		defer wg.Done()
		for n := range jobs {
			if err := ctx.Err(); err != nil {
				return
			}
			req := buildBundleRequest(scenario, n)
			sentAt := time.Now()
			rec, err := r.Target.Submit(ctx, req, clientIDFor(scenario, n, r.ClientID), regionIDFor(scenario, r.RegionID))
			latency := time.Since(sentAt)
			if sentAt.Before(activeStart) || sentAt.After(activeEnd) {
				continue
			}
			mu.Lock()
			samples = append(samples, latency)
			if err != nil {
				failures++
			} else {
				success++
				switch rec.State {
				case model.StateRejected, model.StateDeadLetter:
					rejected++
				default:
					accepted++
				}
			}
			mu.Unlock()
		}
	}
	for i := 0; i < scenario.Concurrency; i++ {
		wg.Add(1)
		go worker(i)
	}
	go func() {
		defer close(jobs)
		if scenario.RatePerSecond <= 0 {
			for i := 0; i < scenario.Requests; i++ {
				select {
				case <-ctx.Done():
					return
				case jobs <- i:
				}
			}
			return
		}
		interval := time.Second / time.Duration(scenario.RatePerSecond)
		if interval <= 0 {
			interval = time.Millisecond
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for i := 0; i < scenario.Requests; i++ {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case <-ctx.Done():
					return
				case jobs <- i:
				}
			}
		}
	}()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	case <-done:
	}
	finished := time.Now().UTC()
	afterHealth, _ := r.Target.Health(ctx)
	afterMetrics, _ := r.Target.Metrics(ctx)
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	res := Result{
		Scenario:      scenario.Name,
		Mode:          scenario.Mode,
		StartedAt:     start,
		FinishedAt:    finished,
		Duration:      finished.Sub(start),
		Requests:      success + failures,
		Successes:     success,
		Failures:      failures,
		Accepted:      accepted,
		Rejected:      rejected,
		HealthBefore:  beforeHealth,
		HealthAfter:   afterHealth,
		MetricsBefore: beforeMetrics,
		MetricsAfter:  afterMetrics,
	}
	if len(samples) > 0 {
		sum := time.Duration(0)
		for _, v := range samples {
			sum += v
		}
		res.MeanLatency = sum / time.Duration(len(samples))
		res.P95Latency = percentile(samples, 95)
		res.P99Latency = percentile(samples, 99)
		res.MaxLatency = samples[len(samples)-1]
	}
	res.PhaseStats = map[string]PhaseStats{
		"backend":    phaseStatsFromLatency(afterMetrics.BackendLatencyMS),
		"state":      phaseStatsFromLatency(afterMetrics.StateLatencyMS),
		"broker":     phaseStatsFromLatency(afterMetrics.BrokerLatencyMS),
		"wal":        phaseStatsFromLatency(afterMetrics.WALLatencyMS),
		"checkpoint": phaseStatsFromLatency(afterMetrics.CheckpointLatencyMS),
	}
	return res, nil
}

func buildBundleRequest(s Scenario, id int) model.JSONRPCRequest {
	key := id
	if s.DuplicateEvery > 0 && id > 0 && id%s.DuplicateEvery == 0 {
		key = id - 1
	}
	txs := make([]string, 0, s.TxCount)
	for i := 0; i < s.TxCount; i++ {
		txs = append(txs, makeTxHex(s.TxBytes, s.Name, key, i))
	}
	return model.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      int64(id + 1),
		Method:  "eth_sendBundle",
		Params: []model.BundleRequest{{
			Txs:          txs,
			BlockNumber:  "0x1",
			MinTimestamp: time.Now().UTC().Unix(),
			MaxTimestamp: time.Now().UTC().Add(2 * time.Second).Unix(),
		}},
	}
}

func makeTxHex(targetBytes int, scenario string, reqID, txIndex int) string {
	if targetBytes <= 0 {
		targetBytes = 64
	}
	seed := fmt.Sprintf("%s:%d:%d:", scenario, reqID, txIndex)
	buf := make([]byte, 0, 2+targetBytes)
	buf = append(buf, '0', 'x')
	for len(buf)-2 < targetBytes {
		for i := 0; i < len(seed) && len(buf)-2 < targetBytes; i++ {
			buf = append(buf, seed[i])
		}
	}
	return string(buf)
}

func clientIDFor(s Scenario, n int, fallback string) string {
	if s.ClientID != "" {
		return s.ClientID
	}
	if fallback != "" {
		return fallback
	}
	return fmt.Sprintf("client-%d", n%8)
}

func regionIDFor(s Scenario, fallback string) string {
	if s.RegionID != "" {
		return s.RegionID
	}
	if fallback != "" {
		return fallback
	}
	return "local"
}

func percentile(sorted []time.Duration, pct int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	if pct <= 0 {
		return sorted[0]
	}
	if pct >= 100 {
		return sorted[len(sorted)-1]
	}
	idx := int(math.Ceil(float64(pct)/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func phaseStatsFromLatency(ms uint64) PhaseStats {
	if ms == 0 {
		return PhaseStats{}
	}
	d := time.Duration(ms) * time.Millisecond
	return PhaseStats{Count: 1, Mean: d, P95: d, P99: d, Max: d}
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
