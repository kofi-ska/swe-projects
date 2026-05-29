package service

import (
	"context"
	"sync/atomic"
	"time"

	"txpool-builder/v2/internal/model"
)

// processJob keeps the expensive work off the admission path.
func (s *Service) processJob(ctx context.Context, workerID int, env *jobEnvelope) {
	_ = workerID
	startedAt := time.Now().UTC()

	s.mu.Lock()
	job, ok := s.jobs[env.JobID]
	if !ok {
		s.mu.Unlock()
		return
	}
	job.State = model.JobRunning
	job.StartedAt = startedAt
	s.mu.Unlock()

	snapRecord := s.snapshotRecordByID(job.SnapshotID)
	if snapRecord == nil || snapRecord.Snapshot == nil {
		s.failJob(job.JobID, model.ReasonSnapshotTooLarge, "snapshot no longer retained")
		atomic.AddInt64(&s.buildsFailed, 1)
		return
	}

	candidate, trace, err := s.buildCandidate(ctx, env.Request, snapRecord.Snapshot, snapRecord.Decisions, startedAt)
	if err != nil {
		s.failJob(job.JobID, model.ReasonInvariantFailure, err.Error())
		atomic.AddInt64(&s.buildsFailed, 1)
		return
	}

	var artifactURI, traceURI string
	if !s.cfg.NoWrite {
		artifactURI, traceURI, err = persistBuildArtifacts(s.cfg, candidate, trace, snapRecord.Snapshot)
		if err != nil {
			s.failJob(job.JobID, model.ReasonArtifactWriteFailed, err.Error())
			atomic.AddInt64(&s.buildsFailed, 1)
			return
		}
	}

	now := time.Now().UTC()
	result := &model.Result{
		JobID:       job.JobID,
		RequestID:   job.RequestID,
		SnapshotID:  snapRecord.Snapshot.SnapshotID,
		ArtifactURI: artifactURI,
		TraceURI:    traceURI,
		Candidate:   candidate,
		Trace:       trace,
		State:       model.JobCompleted,
		CreatedAt:   job.CreatedAt,
		CompletedAt: now,
	}
	s.recordResult(result)
	s.completeJob(job.JobID)
	atomic.AddInt64(&s.buildsCompleted, 1)
}
