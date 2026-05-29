package measurement

import (
	"context"
	"testing"
	"time"

	"mevrelayv3/internal/model"
	"mevrelayv3/internal/relay"
	"mevrelayv3/internal/telemetry"
)

type fakeTarget struct{}

func (fakeTarget) Submit(context.Context, model.JSONRPCRequest, string, string) (model.BundleRecord, error) {
	return model.BundleRecord{State: model.StateForwarded}, nil
}

func (fakeTarget) Health(context.Context) (relay.HealthReport, error) {
	return relay.HealthReport{State: relay.HealthStateHealthy, Ready: true}, nil
}

func (fakeTarget) Metrics(context.Context) (telemetry.Snapshot, error) {
	return telemetry.Snapshot{PolicyConfidence: 1, PolicyPressure: 0.1}, nil
}

func TestRunner(t *testing.T) {
	runner := Runner{Target: fakeTarget{}, ClientID: "c1", RegionID: "r1"}
	rep, err := runner.Run(context.Background(), Scenario{
		Name:          "steady",
		Mode:          ModeSteady,
		Duration:      200 * time.Millisecond,
		Concurrency:   2,
		RatePerSecond: 20,
		TxCount:       2,
		TxBytes:       32,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Results) != 1 {
		t.Fatalf("expected one result, got %d", len(rep.Results))
	}
	if rep.Results[0].Requests == 0 {
		t.Fatal("expected requests to be recorded")
	}
}
