package backend

import (
	"context"

	"mevrelayv1/internal/model"
)

// Simulator executes and health-checks the configured simulation backend.
type Simulator interface {
	Simulate(ctx context.Context, rec model.BundleRecord) (model.SimulationResult, error)
	Ping(ctx context.Context) error
}
