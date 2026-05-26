package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"txpool-builder/v2/internal/model"
)

// Submit keeps admission cheap so hot-path work does not scale with build cost.
func (s *Service) Submit(ctx context.Context, req model.BuildRequest) (model.BuildResponse, int, error) {
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return model.BuildResponse{}, http.StatusBadRequest, fmt.Errorf("idempotency_key is required")
	}
	req.PriorityClass = normalizePriorityClass(req.PriorityClass)
	if req.PolicyVersion == "" {
		req.PolicyVersion = s.cfg.PolicyVersion
	}
	reqID := deterministicID("request", req.IdempotencyKey, req.PriorityClass, req.PolicyVersion)

	s.mu.Lock()
	if rec, ok := s.idempotency[req.IdempotencyKey]; ok && time.Now().Before(rec.ExpiresAt) {
		if job, ok := s.jobs[rec.JobID]; ok {
			resp := model.BuildResponse{
				RequestID:    reqID,
				JobID:        job.JobID,
				State:        string(job.State),
				ReasonCode:   job.ReasonCode,
				ReasonDetail: job.ReasonDetail,
				RetryAfterMS: job.RetryAfterMS,
				SnapshotID:   job.SnapshotID,
			}
			if res, ok := s.results[job.JobID]; ok {
				resp.ResultURI = res.ArtifactURI
				resp.TraceURI = res.TraceURI
			}
			s.mu.Unlock()
			return resp, http.StatusAccepted, nil
		}
	}
	snapshot := s.currentSnapshot()
	if snapshot == nil {
		s.mu.Unlock()
		return model.BuildResponse{}, http.StatusServiceUnavailable, fmt.Errorf("no fresh snapshot available")
	}
	if !snapshotFreshEnough(snapshot, s.cfg.MaxSnapshotAge) {
		s.mu.Unlock()
		return model.BuildResponse{}, http.StatusServiceUnavailable, fmt.Errorf("snapshot too old")
	}
	if len(s.queue) >= cap(s.queue) {
		atomic.AddInt64(&s.shedCount, 1)
		s.mu.Unlock()
		return model.BuildResponse{}, http.StatusTooManyRequests, fmt.Errorf("queue full")
	}

	jobID := deterministicID("job", reqID, req.IdempotencyKey, req.PriorityClass, snapshot.SnapshotID)
	job := &model.JobRecord{
		JobID:          jobID,
		RequestID:      reqID,
		IdempotencyKey: req.IdempotencyKey,
		PriorityClass:  req.PriorityClass,
		PolicyVersion:  req.PolicyVersion,
		State:          model.JobQueued,
		SnapshotID:     snapshot.SnapshotID,
		CreatedAt:      time.Now().UTC(),
	}
	s.jobs[jobID] = job
	s.idempotency[req.IdempotencyKey] = &idempotencyRecord{
		JobID:     jobID,
		RequestID: reqID,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(30 * time.Minute),
	}
	s.pruneLocked()
	s.mu.Unlock()

	select {
	case s.queue <- &jobEnvelope{Request: req, JobID: jobID}:
		return model.BuildResponse{RequestID: reqID, JobID: jobID, State: string(model.JobQueued), SnapshotID: snapshot.SnapshotID}, http.StatusAccepted, nil
	default:
		atomic.AddInt64(&s.shedCount, 1)
		s.mu.Lock()
		if j, ok := s.jobs[jobID]; ok {
			j.State = model.JobShed
			j.ReasonCode = model.ReasonCapacityExcluded
			j.ReasonDetail = "queue full"
			j.CompletedAt = time.Now().UTC()
		}
		s.mu.Unlock()
		return model.BuildResponse{RequestID: reqID, JobID: jobID, State: string(model.JobShed), ReasonCode: model.ReasonCapacityExcluded, ReasonDetail: "queue full", SnapshotID: snapshot.SnapshotID}, http.StatusTooManyRequests, nil
	}
}

// workerLoop exists to keep queue consumption isolated from HTTP admission.
func (s *Service) workerLoop(workerID int) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case env := <-s.queue:
			if env == nil {
				continue
			}
			s.processJob(s.ctx, workerID, env)
		}
	}
}

// refreshLoop refreshes the shared snapshot on cadence so workers can reuse it.
func (s *Service) refreshLoop() {
	ticker := time.NewTicker(s.cfg.RefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.refreshSnapshot(s.ctx); err != nil {
				s.log.Warn("snapshot_refresh_failed", "err", err.Error())
			}
		}
	}
}

// normalizePriorityClass collapses request variance into the few classes the queue can honor.
func normalizePriorityClass(class string) string {
	switch strings.ToLower(strings.TrimSpace(class)) {
	case "high":
		return "high"
	case "low":
		return "low"
	default:
		return "normal"
	}
}
