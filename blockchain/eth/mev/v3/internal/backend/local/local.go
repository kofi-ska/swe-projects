package local

import (
	"context"

	"mevrelayv3/internal/backend"
	"mevrelayv3/internal/model"
)

type Adapter struct{}

func New() *Adapter { return &Adapter{} }

func (a *Adapter) Simulate(ctx context.Context, rec model.BundleRecord) (backend.Result, error) {
	_ = ctx
	success := rec.ExpectedValue-rec.ExpectedCost > 0
	return backend.Result{
		Score:     1,
		ProfitEth: rec.ExpectedValue - rec.ExpectedCost,
		Success:   success,
		Reason:    "local simulation",
	}, nil
}

func (a *Adapter) Ping(context.Context) error { return nil }
func (a *Adapter) Close() error               { return nil }
