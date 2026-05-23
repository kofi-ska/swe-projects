package memory

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"mevrelayv2/internal/lifecycle"
	"mevrelayv2/internal/model"
)

// Store is an in-memory v2 state store for tests and isolated runs.
type Store struct {
	mu          sync.RWMutex
	records     map[string]model.BundleRecord
	hashIndex   map[string]string
	events      map[string][]model.EventRecord
	checkpoints map[string]model.CheckpointRecord
	inflight    map[string]int
	retries     map[string]time.Time
	limit       int
	closed      bool
}

// New creates a memory-backed store.
func New() *Store {
	return &Store{
		records:     make(map[string]model.BundleRecord),
		hashIndex:   make(map[string]string),
		events:      make(map[string][]model.EventRecord),
		checkpoints: make(map[string]model.CheckpointRecord),
		inflight:    make(map[string]int),
		retries:     make(map[string]time.Time),
		limit:       256,
	}
}

func (s *Store) CreateBundle(ctx context.Context, rec model.BundleRecord) (model.BundleRecord, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return model.BundleRecord{}, errors.New("state closed")
	}
	if existing, ok := s.hashIndex[rec.BundleHash]; ok {
		return s.records[existing], errors.New("duplicate bundle")
	}
	s.records[rec.ID] = rec
	s.hashIndex[rec.BundleHash] = rec.ID
	s.pruneLocked()
	return rec, nil
}

func (s *Store) GetBundle(ctx context.Context, id string) (model.BundleRecord, bool, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.records[id]
	return rec, ok, nil
}

func (s *Store) ListBundles(ctx context.Context, limit int) ([]model.BundleRecord, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > s.limit {
		limit = s.limit
	}
	out := make([]model.BundleRecord, 0, len(s.records))
	for _, rec := range s.records {
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

func (s *Store) TransitionBundle(ctx context.Context, id string, from, to model.BundleState, reason string) (model.BundleRecord, error) {
	_ = ctx
	if err := lifecycle.ValidateTransition(from, to); err != nil {
		return model.BundleRecord{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		return model.BundleRecord{}, errors.New("bundle not found")
	}
	if rec.State != from {
		return model.BundleRecord{}, errors.New("state mismatch")
	}
	rec.State = to
	rec.Reason = reason
	rec.Version++
	rec.Sequence++
	rec.UpdatedAt = time.Now().UTC()
	switch to {
	case model.StateQueued:
		rec.QueuedAt = rec.UpdatedAt
	case model.StateCompleted:
		rec.CompletedAt = rec.UpdatedAt
	}
	switch to {
	case model.StateForwarded, model.StateRejected, model.StateDeadLetter:
		rec.Terminal = string(to)
	}
	s.records[id] = rec
	return rec, nil
}

func (s *Store) UpdateRetryCount(ctx context.Context, id string, retryCount int) (model.BundleRecord, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		return model.BundleRecord{}, errors.New("bundle not found")
	}
	rec.RetryCount = retryCount
	rec.Version++
	rec.UpdatedAt = time.Now().UTC()
	s.records[id] = rec
	return rec, nil
}

func (s *Store) UpdateResult(ctx context.Context, id string, score, profit float64, reason string) (model.BundleRecord, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.records[id]
	if !ok {
		return model.BundleRecord{}, errors.New("bundle not found")
	}
	rec.Score = score
	rec.ProfitEth = profit
	rec.Reason = reason
	rec.Version++
	rec.UpdatedAt = time.Now().UTC()
	s.records[id] = rec
	return rec, nil
}

func (s *Store) ReserveInflight(ctx context.Context, clientID string, limit int) (int, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.inflight[clientID]
	if cur >= limit {
		return cur, errors.New("client inflight limit")
	}
	if cur == 0 && len(s.inflight) >= s.limit {
		return cur, errors.New("state capacity")
	}
	cur++
	s.inflight[clientID] = cur
	return cur, nil
}

func (s *Store) ReleaseInflight(ctx context.Context, clientID string) (int, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.inflight[clientID]
	if cur > 0 {
		cur--
		if cur == 0 {
			delete(s.inflight, clientID)
		} else {
			s.inflight[clientID] = cur
		}
	}
	return cur, nil
}

func (s *Store) GetInflight(ctx context.Context, clientID string) (int, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inflight[clientID], nil
}

func (s *Store) ScheduleRetry(ctx context.Context, id string, due time.Time) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("state closed")
	}
	s.retries[id] = due.UTC()
	s.pruneRetriesLocked()
	return nil
}

func (s *Store) ClaimDueRetries(ctx context.Context, now time.Time, limit int) ([]string, error) {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("state closed")
	}
	out := make([]string, 0, limit)
	for id, due := range s.retries {
		if len(out) >= limit {
			break
		}
		if !due.After(now.UTC()) {
			out = append(out, id)
			delete(s.retries, id)
		}
	}
	return out, nil
}

func (s *Store) AppendEvent(ctx context.Context, ev model.EventRecord) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	evs := append(s.events[ev.BundleID], ev)
	if len(evs) > s.limit {
		evs = evs[len(evs)-s.limit:]
	}
	s.events[ev.BundleID] = evs
	return nil
}

func (s *Store) ListEvents(ctx context.Context, bundleID string, limit int) ([]model.EventRecord, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > s.limit {
		limit = s.limit
	}
	src := s.events[bundleID]
	if len(src) > limit {
		src = src[len(src)-limit:]
	}
	out := make([]model.EventRecord, len(src))
	copy(out, src)
	return out, nil
}

func (s *Store) PutCheckpoint(ctx context.Context, cp model.CheckpointRecord) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkpoints[cp.BatchID] = cp
	s.pruneCheckpointsLocked()
	return nil
}

func (s *Store) ListCheckpoints(ctx context.Context, limit int) ([]model.CheckpointRecord, error) {
	_ = ctx
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > s.limit {
		limit = s.limit
	}
	out := make([]model.CheckpointRecord, 0, len(s.checkpoints))
	for _, cp := range s.checkpoints {
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

func (s *Store) DeleteEvents(ctx context.Context, bundleID string) error {
	_ = ctx
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.events, bundleID)
	return nil
}

func (s *Store) Health(context.Context) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return errors.New("state closed")
	}
	return nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *Store) pruneLocked() {
	if len(s.records) <= s.limit {
		return
	}
	var oldestID string
	var oldestTime time.Time
	for id, rec := range s.records {
		if oldestID == "" || rec.CreatedAt.Before(oldestTime) {
			oldestID = id
			oldestTime = rec.CreatedAt
		}
	}
	if oldestID == "" {
		return
	}
	delete(s.hashIndex, s.records[oldestID].BundleHash)
	delete(s.records, oldestID)
	delete(s.events, oldestID)
}

func (s *Store) pruneRetriesLocked() {
	if len(s.retries) <= s.limit {
		return
	}
	var oldestID string
	var oldestDue time.Time
	for id, due := range s.retries {
		if oldestID == "" || due.Before(oldestDue) {
			oldestID = id
			oldestDue = due
		}
	}
	delete(s.retries, oldestID)
}

func (s *Store) pruneCheckpointsLocked() {
	if len(s.checkpoints) <= s.limit {
		return
	}
	var oldestID string
	var oldestTime time.Time
	for id, cp := range s.checkpoints {
		if oldestID == "" || cp.Time.Before(oldestTime) {
			oldestID = id
			oldestTime = cp.Time
		}
	}
	delete(s.checkpoints, oldestID)
}
