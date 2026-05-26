package backend

import (
	"context"

	"mevrelayv3/internal/model"
)

type Kind string

const (
	KindLocal Kind = "local"
)

type Result struct {
	Score     float64
	ProfitEth float64
	Success   bool
	Reason    string
}

type Adapter interface {
	Simulate(context.Context, model.BundleRecord) (Result, error)
	Ping(context.Context) error
	Close() error
}
