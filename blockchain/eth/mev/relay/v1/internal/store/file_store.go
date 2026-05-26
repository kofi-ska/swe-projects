package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"mevrelayv1/internal/lifecycle"
	"mevrelayv1/internal/model"
)

type FileStore struct {
	mu      sync.Mutex
	root    string
	snap    string
	audit   string
	records map[string]model.BundleRecord
}

type snapshot struct {
	Records map[string]model.BundleRecord `json:"records"`
}

// NewFileStore opens or creates the file-backed bundle store.
func NewFileStore(root string) (*FileStore, error) {
	if root == "" {
		root = "data"
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}

	fs := &FileStore{
		root:    root,
		snap:    filepath.Join(root, "state.json"),
		audit:   filepath.Join(root, "audit.jsonl"),
		records: map[string]model.BundleRecord{},
	}
	if err := fs.load(); err != nil {
		return nil, err
	}
	return fs, nil
}

func (fs *FileStore) load() error {
	data, err := os.ReadFile(fs.snap)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	var snap snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	if snap.Records != nil {
		fs.records = snap.Records
	}
	return nil
}

func (fs *FileStore) Create(ctx context.Context, rec model.BundleRecord) (model.BundleRecord, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if _, exists := fs.records[rec.ID]; exists {
		return model.BundleRecord{}, fmt.Errorf("bundle already exists")
	}
	now := time.Now().UTC()
	rec.CreatedAt = now
	rec.UpdatedAt = now
	rec.Version = 1
	fs.records[rec.ID] = rec

	if err := fs.persistLocked(); err != nil {
		return model.BundleRecord{}, err
	}
	if err := fs.appendEvent(model.EventRecord{
		Time:     now,
		BundleID: rec.ID,
		To:       rec.State,
		Version:  rec.Version,
		ClientID: rec.ClientID,
	}); err != nil {
		return model.BundleRecord{}, err
	}
	return rec, nil
}

func (fs *FileStore) Get(ctx context.Context, id string) (model.BundleRecord, bool, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	rec, ok := fs.records[id]
	return rec, ok, nil
}

func (fs *FileStore) Transition(ctx context.Context, id string, from, to model.BundleState, reason string) (model.BundleRecord, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	rec, ok := fs.records[id]
	if !ok {
		return model.BundleRecord{}, fmt.Errorf("bundle not found")
	}
	if rec.State != from {
		return model.BundleRecord{}, fmt.Errorf("state mismatch: have %s want %s", rec.State, from)
	}
	if err := lifecycle.ValidateTransition(from, to); err != nil {
		return model.BundleRecord{}, err
	}
	rec.State = to
	rec.Reason = reason
	rec.Version++
	rec.UpdatedAt = time.Now().UTC()
	switch to {
	case model.StateQueued:
		rec.QueuedAt = rec.UpdatedAt
	case model.StateSimulated:
		rec.SimulatedAt = rec.UpdatedAt
	case model.StateCompleted:
		rec.CompletedAt = rec.UpdatedAt
	}
	fs.records[id] = rec
	if err := fs.persistLocked(); err != nil {
		return model.BundleRecord{}, err
	}
	if err := fs.appendEvent(model.EventRecord{
		Time:     rec.UpdatedAt,
		BundleID: rec.ID,
		From:     from,
		To:       to,
		Reason:   reason,
		Version:  rec.Version,
		ClientID: rec.ClientID,
	}); err != nil {
		return model.BundleRecord{}, err
	}
	return rec, nil
}

func (fs *FileStore) UpdateRetryCount(ctx context.Context, id string, retryCount int) (model.BundleRecord, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	rec, ok := fs.records[id]
	if !ok {
		return model.BundleRecord{}, fmt.Errorf("bundle not found")
	}
	rec.RetryCount = retryCount
	rec.Version++
	rec.UpdatedAt = time.Now().UTC()
	fs.records[id] = rec
	if err := fs.persistLocked(); err != nil {
		return model.BundleRecord{}, err
	}
	if err := fs.appendEvent(model.EventRecord{
		Time:     rec.UpdatedAt,
		BundleID: rec.ID,
		To:       rec.State,
		Reason:   fmt.Sprintf("retry count %d", retryCount),
		Version:  rec.Version,
		ClientID: rec.ClientID,
	}); err != nil {
		return model.BundleRecord{}, err
	}
	return rec, nil
}

func (fs *FileStore) UpdateResult(ctx context.Context, id string, score, profit float64, reason string) (model.BundleRecord, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	rec, ok := fs.records[id]
	if !ok {
		return model.BundleRecord{}, fmt.Errorf("bundle not found")
	}
	rec.Score = score
	rec.ProfitEth = profit
	rec.Reason = reason
	rec.Version++
	rec.UpdatedAt = time.Now().UTC()
	fs.records[id] = rec
	if err := fs.persistLocked(); err != nil {
		return model.BundleRecord{}, err
	}
	if err := fs.appendEvent(model.EventRecord{
		Time:     rec.UpdatedAt,
		BundleID: rec.ID,
		From:     rec.State,
		To:       rec.State,
		Reason:   "result updated",
		Version:  rec.Version,
		ClientID: rec.ClientID,
	}); err != nil {
		return model.BundleRecord{}, err
	}
	return rec, nil
}

func (fs *FileStore) List(ctx context.Context) ([]model.BundleRecord, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	out := make([]model.BundleRecord, 0, len(fs.records))
	for _, rec := range fs.records {
		out = append(out, rec)
	}
	return out, nil
}

func (fs *FileStore) Health(ctx context.Context) error {
	f, err := os.OpenFile(fs.audit, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}

func (fs *FileStore) Close() error { return nil }

func (fs *FileStore) persistLocked() error {
	snap := snapshot{Records: fs.records}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	tmp := fs.snap + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, fs.snap)
}

func (fs *FileStore) appendEvent(ev model.EventRecord) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(fs.audit, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	return err
}
