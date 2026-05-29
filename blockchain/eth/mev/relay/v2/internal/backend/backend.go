package backend

import (
	"context"

	"mevrelayv2/internal/model"
)

// Kind identifies a supported simulation backend.
type Kind string

const (
	KindLocal   Kind = "local"
	KindAnvil   Kind = "anvil"
	KindSepolia Kind = "sepolia"
)

// Result is the bounded decision signal returned by a backend adapter.
type Result struct {
	Score     float64
	ProfitEth float64
	Success   bool
	Reason    string
}

// Adapter executes and health-checks one backend target.
type Adapter interface {
	Simulate(context.Context, model.BundleRecord) (Result, error)
	Ping(context.Context) error
	Close() error
}
