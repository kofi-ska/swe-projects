package state

import (
	"context"
	"errors"
	"time"

	"mevrelayv2/internal/model"
)

var (
	ErrStateClosed     = errors.New("state closed")
	ErrDuplicateBundle = errors.New("duplicate bundle")
	ErrBundleNotFound  = errors.New("bundle not found")
	ErrStateMismatch   = errors.New("state mismatch")
	ErrClientInflight  = errors.New("client inflight limit")
	ErrStateCapacity   = errors.New("state capacity")
)

// Store owns the authoritative v2 coordination state.
type Store interface {
	CreateBundle(context.Context, model.BundleRecord) (model.BundleRecord, error)
	GetBundle(context.Context, string) (model.BundleRecord, bool, error)
	ListBundles(context.Context, int) ([]model.BundleRecord, error)
	TransitionBundle(context.Context, string, model.BundleState, model.BundleState, string) (model.BundleRecord, error)
	UpdateRetryCount(context.Context, string, int) (model.BundleRecord, error)
	UpdateResult(context.Context, string, float64, float64, string) (model.BundleRecord, error)
	ReserveInflight(context.Context, string, int) (int, error)
	ReleaseInflight(context.Context, string) (int, error)
	GetInflight(context.Context, string) (int, error)
	ScheduleRetry(context.Context, string, time.Time) error
	ClaimDueRetries(context.Context, time.Time, int) ([]string, error)
	AppendEvent(context.Context, model.EventRecord) error
	ListEvents(context.Context, string, int) ([]model.EventRecord, error)
	PutCheckpoint(context.Context, model.CheckpointRecord) error
	ListCheckpoints(context.Context, int) ([]model.CheckpointRecord, error)
	DeleteEvents(context.Context, string) error
	Health(context.Context) error
	Close() error
}
