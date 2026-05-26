package checkpoint

import (
	"context"

	"mevrelayv3/internal/model"
)

type Store interface {
	Put(context.Context, model.CheckpointRecord, []byte) (string, error)
	Get(context.Context, string) ([]byte, error)
	Health(context.Context) error
	Close() error
}
