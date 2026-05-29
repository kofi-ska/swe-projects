package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"mevrelayv3/internal/app"
	"mevrelayv3/internal/config"
	"mevrelayv3/internal/measurement"
	"mevrelayv3/internal/telemetry"
)

func main() {
	var (
		baseURL  = flag.String("target", "", "http target for an already running relay")
		report   = flag.String("report", "", "output report path")
		baseline = flag.String("baseline", "", "baseline report path")
		scenario = flag.String("scenario", "steady", "steady|burst|failure|replay")
		duration = flag.Duration("duration", 20*time.Second, "scenario duration")
		rps      = flag.Int("rps", 50, "request rate per second")
		conc     = flag.Int("concurrency", 8, "concurrent workers")
	)
	flag.Parse()

	ctx := context.Background()
	var target measurement.Target
	if strings.TrimSpace(*baseURL) != "" {
		target = measurement.HTTPTarget{BaseURL: *baseURL}
	} else {
		cfg := config.Load()
		shutdownTrace, err := telemetry.InitTracing(ctx, telemetry.TraceConfig{
			ServiceName: cfg.OTELServiceName,
			Endpoint:    cfg.OTELExporterEndpoint,
			Insecure:    cfg.OTELExporterInsecure,
			SampleRatio: cfg.OTELSampleRatio,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer func() { _ = shutdownTrace(context.Background()) }()
		rt, err := app.Build(cfg)
		if err != nil {
			log.Fatal(err)
		}
		defer rt.Close()
		if err := rt.Service.Bootstrap(ctx); err != nil {
			log.Fatal(err)
		}
		rt.Service.Start(ctx)
		defer rt.Service.Stop()
		target = measurement.EmbeddedTarget{Svc: rt.Service}
	}

	sc := measurement.Scenario{
		Name:          *scenario,
		Mode:          measurement.Mode(strings.ToLower(*scenario)),
		Duration:      *duration,
		Concurrency:   *conc,
		RatePerSecond: *rps,
		TxCount:       3,
		TxBytes:       128,
		Requests:      0,
	}
	scenarios := []measurement.Scenario{sc}
	if strings.EqualFold(*scenario, "all") || strings.EqualFold(*scenario, "suite") {
		scenarios = []measurement.Scenario{
			{Name: "steady", Mode: measurement.ModeSteady, Duration: 20 * time.Second, Warmup: 2 * time.Second, Cooldown: 2 * time.Second, Concurrency: *conc, RatePerSecond: *rps, TxCount: 3, TxBytes: 128},
			{Name: "burst", Mode: measurement.ModeBurst, Duration: 10 * time.Second, Warmup: 0, Cooldown: 0, Concurrency: *conc * 2, RatePerSecond: 0, TxCount: 4, TxBytes: 128},
			{Name: "failure", Mode: measurement.ModeFailure, Duration: 12 * time.Second, Warmup: time.Second, Cooldown: time.Second, Concurrency: *conc, RatePerSecond: *rps, TxCount: 2, TxBytes: 64, DuplicateEvery: 3},
		}
	}
	rep := measurement.Report{GeneratedAt: time.Now().UTC()}
	for _, scn := range scenarios {
		activeTarget := target
		if scn.Mode == measurement.ModeFailure {
			activeTarget = measurement.FaultTarget{
				Target: activeTarget,
				Plan: measurement.FaultPlan{
					SubmitDelay:       25 * time.Millisecond,
					HealthDelay:       50 * time.Millisecond,
					MetricsDelay:      50 * time.Millisecond,
					SubmitErrorEvery:  7,
					HealthErrorEvery:  11,
					MetricsErrorEvery: 13,
				},
			}
		}
		runner := measurement.Runner{Target: activeTarget}
		sub, err := runner.Run(ctx, scn)
		if err != nil {
			log.Fatal(err)
		}
		rep.Results = append(rep.Results, sub.Results...)
	}
	if *baseline != "" {
		base, err := measurement.LoadBaseline(*baseline)
		if err != nil {
			log.Fatal(err)
		}
		for i := range rep.Results {
			if base.Name != "" && base.Name != rep.Results[i].Scenario {
				continue
			}
			ok, reasons := measurement.Compare(rep.Results[i], base)
			rep.Results[i].Regression = !ok
			rep.Results[i].RegressionWhy = reasons
		}
	}
	if *report != "" {
		if err := measurement.SaveReport(*report, rep); err != nil {
			log.Fatal(err)
		}
	}
	if *report == "" {
		if err := json.NewEncoder(os.Stdout).Encode(rep); err != nil {
			log.Fatal(err)
		}
	}
}
