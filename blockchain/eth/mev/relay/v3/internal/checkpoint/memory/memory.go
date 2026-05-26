package memory

import (
	"context"
	"errors"
	"sync"

	"mevrelayv3/internal/checkpoint"
	"mevrelayv3/internal/model"
)

type Store struct {
	mu     sync.RWMutex
	blobs  map[string][]byte
	closed bool
}

func New() *Store {
	return &Store{blobs: map[string][]byte{}}
}

func (s *Store) Put(_ context.Context, cp model.CheckpointRecord, body []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", errors.New("checkpoint store closed")
	}
	key := cp.ObjectKey
	if key == "" {
		key = cp.BatchID + ".json"
	}
	s.blobs[key] = append([]byte(nil), body...)
	return key, nil
}

func (s *Store) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	body, ok := s.blobs[key]
	if !ok {
		return nil, errors.New("checkpoint not found")
	}
	return append([]byte(nil), body...), nil
}

func (s *Store) Health(context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return errors.New("checkpoint store closed")
	}
	return nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

var _ checkpoint.Store = (*Store)(nil)
