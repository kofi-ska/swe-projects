package relay

import (
	"context"
	"errors"
	"sync"
	"time"

	"mevrelayv3/internal/config"
)

var ErrPolicyStoreClosed = errors.New("policy store closed")

type PolicyStore interface {
	Load(context.Context, string) (PolicySnapshot, bool, error)
	Save(context.Context, string, PolicySnapshot) error
	Health(context.Context) error
	Close() error
}

type policyMemoryStore struct {
	mu     sync.RWMutex
	closed bool
	data   map[string]PolicySnapshot
}

var _ PolicyStore = (*policyMemoryStore)(nil)

func newPolicyStore(cfg config.Config) PolicyStore {
	if cfg.StateKind == "valkey" {
		if st, err := newPolicyValkeyStore(cfg.ValkeyURL, cfg.StateRetention); err == nil {
			return st
		}
	}
	return newPolicyMemoryStore()
}

func newPolicyMemoryStore() PolicyStore {
	return &policyMemoryStore{data: map[string]PolicySnapshot{}}
}

func (s *policyMemoryStore) Load(_ context.Context, shardID string) (PolicySnapshot, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return PolicySnapshot{}, false, ErrPolicyStoreClosed
	}
	snap, ok := s.data[shardID]
	return snap, ok, nil
}

func (s *policyMemoryStore) Save(_ context.Context, shardID string, snap PolicySnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrPolicyStoreClosed
	}
	s.data[shardID] = snap
	return nil
}

func (s *policyMemoryStore) Health(context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return ErrPolicyStoreClosed
	}
	return nil
}

func (s *policyMemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func policyTTL(d time.Duration) time.Duration {
	if d <= 0 {
		return 24 * time.Hour
	}
	return d
}
