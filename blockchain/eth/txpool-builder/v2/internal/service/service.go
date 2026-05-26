package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"txpool-builder/v2/internal/model"
	rpcx "txpool-builder/v2/internal/rpc"
)

type Service struct {
	cfg model.Config
	rpc rpcx.Caller
	log *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	queue chan *jobEnvelope

	mu          sync.RWMutex
	jobs        map[string]*model.JobRecord
	results     map[string]*model.Result
	idempotency map[string]*idempotencyRecord
	snapshots   []*snapshotRecord

	snapshotMu sync.RWMutex
	current    atomic.Pointer[model.Snapshot]

	buildsCompleted int64
	buildsFailed    int64
	shedCount       int64
}

type idempotencyRecord struct {
	JobID     string
	RequestID string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type snapshotRecord struct {
	Snapshot  *model.Snapshot
	Decisions []model.TxDecision
	CreatedAt time.Time
	Path      string
}

type jobEnvelope struct {
	Request model.BuildRequest
	JobID   string
}

func New(cfg model.Config, rpc rpcx.Caller, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		cfg:         cfg,
		rpc:         rpc,
		log:         logger,
		queue:       make(chan *jobEnvelope, cfg.QueueSize),
		jobs:        map[string]*model.JobRecord{},
		results:     map[string]*model.Result{},
		idempotency: map[string]*idempotencyRecord{},
		snapshots:   []*snapshotRecord{},
	}
}

func (s *Service) Start(ctx context.Context) error {
	s.ctx, s.cancel = context.WithCancel(ctx)
	if err := s.refreshSnapshot(s.ctx); err != nil {
		return err
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.refreshLoop()
	}()
	for i := 0; i < s.cfg.WorkerCount; i++ {
		s.wg.Add(1)
		go func(workerID int) {
			defer s.wg.Done()
			s.workerLoop(workerID)
		}(i)
	}
	return nil
}

func (s *Service) Shutdown(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.wg.Wait()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func (s *Service) Status() model.Status {
	snapshot := s.currentSnapshot()
	st := model.Status{
		Healthy:         snapshot != nil,
		Mode:            "normal",
		QueueDepth:      len(s.queue),
		WorkerCount:     s.cfg.WorkerCount,
		BuildsCompleted: atomic.LoadInt64(&s.buildsCompleted),
		BuildsFailed:    atomic.LoadInt64(&s.buildsFailed),
		ShedCount:       atomic.LoadInt64(&s.shedCount),
		UpdatedAt:       time.Now().UTC(),
	}
	if snapshot != nil {
		st.SnapshotID = snapshot.SnapshotID
		st.SnapshotAgeMS = time.Since(snapshot.CapturedAt).Milliseconds()
		st.LastRefreshMS = snapshot.RefreshMS
		if st.SnapshotAgeMS > s.cfg.MaxSnapshotAge.Milliseconds() {
			st.Mode = "degraded"
		}
	}
	if st.QueueDepth >= s.cfg.QueueSize {
		st.Mode = "degraded"
	}
	return st
}

func (s *Service) currentSnapshot() *model.Snapshot {
	s.snapshotMu.RLock()
	defer s.snapshotMu.RUnlock()
	return s.current.Load()
}

func (s *Service) setSnapshot(snap *model.Snapshot) {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	s.current.Store(snap)
}

func (s *Service) snapshotByID(snapshotID string) *model.Snapshot {
	if snapshotID == "" {
		return nil
	}
	s.snapshotMu.RLock()
	defer s.snapshotMu.RUnlock()
	if current := s.current.Load(); current != nil && current.SnapshotID == snapshotID {
		return current
	}
	for i := len(s.snapshots) - 1; i >= 0; i-- {
		if s.snapshots[i] != nil && s.snapshots[i].Snapshot != nil && s.snapshots[i].Snapshot.SnapshotID == snapshotID {
			return s.snapshots[i].Snapshot
		}
	}
	return nil
}

func (s *Service) snapshotRecordByID(snapshotID string) *snapshotRecord {
	if snapshotID == "" {
		return nil
	}
	s.snapshotMu.RLock()
	defer s.snapshotMu.RUnlock()
	for i := len(s.snapshots) - 1; i >= 0; i-- {
		rec := s.snapshots[i]
		if rec != nil && rec.Snapshot != nil && rec.Snapshot.SnapshotID == snapshotID {
			return rec
		}
	}
	return nil
}

func (s *Service) recordSnapshot(snap *model.Snapshot, decisions []model.TxDecision, path string) {
	s.snapshotMu.Lock()
	defer s.snapshotMu.Unlock()
	s.current.Store(snap)
	s.snapshots = append(s.snapshots, &snapshotRecord{Snapshot: snap, Decisions: decisions, CreatedAt: time.Now().UTC(), Path: path})
	if len(s.snapshots) <= s.cfg.MaxRetainedSnap {
		return
	}
	keep := s.snapshots[len(s.snapshots)-s.cfg.MaxRetainedSnap:]
	s.snapshots = append([]*snapshotRecord(nil), keep...)
}

func (s *Service) recordResult(res *model.Result) {
	if res == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[res.JobID] = res
}

func (s *Service) failJob(jobID string, code model.ReasonCode, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.jobs[jobID]; ok {
		job.State = model.JobFailed
		job.ReasonCode = code
		job.ReasonDetail = detail
		job.CompletedAt = time.Now().UTC()
	}
}

func (s *Service) completeJob(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, ok := s.jobs[jobID]; ok {
		job.State = model.JobCompleted
		job.CompletedAt = time.Now().UTC()
		job.ReasonCode = ""
		job.ReasonDetail = ""
	}
}

func (s *Service) pruneLocked() {
	if s.cfg.MaxRetainedJobs <= 0 {
		return
	}
	if len(s.jobs) <= s.cfg.MaxRetainedJobs {
		s.pruneIdempotencyLocked()
		return
	}
	completed := make([]*model.JobRecord, 0, len(s.jobs))
	for _, j := range s.jobs {
		if j.State == model.JobCompleted || j.State == model.JobFailed || j.State == model.JobShed {
			completed = append(completed, j)
		}
	}
	if len(completed) == 0 {
		s.pruneIdempotencyLocked()
		return
	}
	sortJobsByTime(completed)
	for len(s.jobs) > s.cfg.MaxRetainedJobs && len(completed) > 0 {
		old := completed[0]
		completed = completed[1:]
		delete(s.jobs, old.JobID)
		delete(s.results, old.JobID)
		delete(s.idempotency, old.IdempotencyKey)
	}
	s.pruneIdempotencyLocked()
}

func (s *Service) pruneIdempotencyLocked() {
	now := time.Now().UTC()
	for key, rec := range s.idempotency {
		if rec == nil || now.After(rec.ExpiresAt) {
			delete(s.idempotency, key)
		}
	}
}

func sortJobsByTime(jobs []*model.JobRecord) {
	sort.SliceStable(jobs, func(i, j int) bool {
		if jobs[i].CompletedAt.Equal(jobs[j].CompletedAt) {
			return jobs[i].CreatedAt.Before(jobs[j].CreatedAt)
		}
		return jobs[i].CompletedAt.Before(jobs[j].CompletedAt)
	})
}

func snapshotFreshEnough(snap *model.Snapshot, maxAge time.Duration) bool {
	return snap != nil && snap.SnapshotID != "" && time.Since(snap.CapturedAt) <= maxAge
}

func deterministicID(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:24]
}
