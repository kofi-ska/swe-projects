package local

import (
	"context"
	"errors"

	"mevrelayv2/internal/backend"
	"mevrelayv2/internal/model"
)

// Adapter is the in-process deterministic backend used for local runs and tests.
type Adapter struct{}

// New creates a local backend adapter.
func New() *Adapter {
	return &Adapter{}
}

// Simulate produces a deterministic score without external RPC calls.
func (a *Adapter) Simulate(ctx context.Context, rec model.BundleRecord) (backend.Result, error) {
	if err := ctx.Err(); err != nil {
		return backend.Result{}, err
	}
	if len(rec.Request.Txs) == 0 {
		return backend.Result{}, errors.New("missing txs")
	}
	count := len(rec.Request.Txs)
	if count%5 == 0 {
		return backend.Result{}, retryableError{msg: "transient simulation pressure"}
	}
	return backend.Result{
		Score:     float64(count),
		ProfitEth: float64(count) / 10,
		Success:   count%2 == 0,
		Reason:    "deterministic local simulation",
	}, nil
}

// Ping reports that the local backend is always available.
func (a *Adapter) Ping(context.Context) error { return nil }

// Close releases local resources.
func (a *Adapter) Close() error { return nil }

type retryableError struct{ msg string }

func (e retryableError) Error() string   { return e.msg }
func (e retryableError) Retryable() bool { return true }
