package measurement

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"mevrelayv3/internal/model"
	"mevrelayv3/internal/relay"
	"mevrelayv3/internal/telemetry"
)

type FaultPlan struct {
	SubmitDelay       time.Duration
	HealthDelay       time.Duration
	MetricsDelay      time.Duration
	SubmitErrorEvery  int
	HealthErrorEvery  int
	MetricsErrorEvery int
}

type FaultTarget struct {
	Target  Target
	Plan    FaultPlan
	submit  atomic.Uint64
	health  atomic.Uint64
	metrics atomic.Uint64
}

func (t FaultTarget) Submit(ctx context.Context, req model.JSONRPCRequest, clientID, regionID string) (model.BundleRecord, error) {
	n := t.submit.Add(1)
	if t.Plan.SubmitErrorEvery > 0 && int(n)%t.Plan.SubmitErrorEvery == 0 {
		return model.BundleRecord{}, errors.New("injected submit failure")
	}
	if t.Plan.SubmitDelay > 0 {
		select {
		case <-ctx.Done():
			return model.BundleRecord{}, ctx.Err()
		case <-time.After(t.Plan.SubmitDelay):
		}
	}
	return t.Target.Submit(ctx, req, clientID, regionID)
}

func (t FaultTarget) Health(ctx context.Context) (relay.HealthReport, error) {
	n := t.health.Add(1)
	if t.Plan.HealthErrorEvery > 0 && int(n)%t.Plan.HealthErrorEvery == 0 {
		return relay.HealthReport{}, errors.New("injected health failure")
	}
	if t.Plan.HealthDelay > 0 {
		select {
		case <-ctx.Done():
			return relay.HealthReport{}, ctx.Err()
		case <-time.After(t.Plan.HealthDelay):
		}
	}
	return t.Target.Health(ctx)
}

func (t FaultTarget) Metrics(ctx context.Context) (telemetry.Snapshot, error) {
	n := t.metrics.Add(1)
	if t.Plan.MetricsErrorEvery > 0 && int(n)%t.Plan.MetricsErrorEvery == 0 {
		return telemetry.Snapshot{}, errors.New("injected metrics failure")
	}
	if t.Plan.MetricsDelay > 0 {
		select {
		case <-ctx.Done():
			return telemetry.Snapshot{}, ctx.Err()
		case <-time.After(t.Plan.MetricsDelay):
		}
	}
	return t.Target.Metrics(ctx)
}
