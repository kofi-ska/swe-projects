package store

import (
	"context"

	"mevrelayv1/internal/model"
)

// Store persists bundle lifecycle state and audit records.
type Store interface {
	Create(ctx context.Context, rec model.BundleRecord) (model.BundleRecord, error)
	Get(ctx context.Context, id string) (model.BundleRecord, bool, error)
	Transition(ctx context.Context, id string, from, to model.BundleState, reason string) (model.BundleRecord, error)
	UpdateRetryCount(ctx context.Context, id string, retryCount int) (model.BundleRecord, error)
	UpdateResult(ctx context.Context, id string, score, profit float64, reason string) (model.BundleRecord, error)
	List(ctx context.Context) ([]model.BundleRecord, error)
	Health(ctx context.Context) error
	Close() error
}
