package relay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"mevrelayv3/internal/broker"
	"mevrelayv3/internal/commitment"
	"mevrelayv3/internal/graph"
	"mevrelayv3/internal/model"
)

func (s *Service) emitTransition(ctx context.Context, rec model.BundleRecord, from, to model.BundleState, reason string) error {
	ctx, span := s.startSpan(ctx, "relay.transition",
		attribute.String("bundle.id", rec.ID),
		attribute.String("shard.id", rec.ShardID),
		attribute.String("state.from", string(from)),
		attribute.String("state.to", string(to)),
	)
	defer span.End()
	auth := s.currentAuthority()
	ev := model.EventRecord{
		Time:       nowUTC(),
		BundleID:   rec.ID,
		BundleHash: rec.BundleHash,
		From:       from,
		To:         to,
		Reason:     reason,
		Version:    rec.Version,
		Sequence:   rec.Sequence,
		ClientID:   rec.ClientID,
		RegionID:   rec.RegionID,
		ShardID:    rec.ShardID,
		LeaseID:    auth.LeaseID,
		LeaseEpoch: auth.Epoch,
		FenceToken: auth.FenceToken,
	}
	start := time.Now()
	if err := s.state.AppendEvent(ctx, ev); err != nil {
		s.metrics.IncStateError()
		endSpan(span, err)
		return err
	}
	s.metrics.ObserveStateLatency(time.Since(start))
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	start = time.Now()
	if err := s.wal.AppendEncoded(ctx, "event", body); err != nil {
		s.metrics.IncWALError()
		endSpan(span, err)
		return err
	}
	s.metrics.ObserveWALLatency(time.Since(start))
	start = time.Now()
	err = s.broker.Publish(ctx, broker.Message{
		Topic:     s.cfg.BrokerTopic + ".event",
		Key:       rec.ID,
		Sequence:  ev.Sequence,
		Timestamp: ev.Time,
		Headers:   map[string]string{"kind": "event"},
		Payload:   body,
	})
	s.metrics.ObserveBrokerLatency(time.Since(start))
	endSpan(span, err)
	return err
}

func (s *Service) finishTerminal(ctx context.Context, rec model.BundleRecord, terminal model.BundleState, reason string) (model.BundleRecord, error) {
	ctx, span := s.startSpan(ctx, "relay.terminalize",
		attribute.String("bundle.id", rec.ID),
		attribute.String("shard.id", rec.ShardID),
		attribute.String("state.terminal", string(terminal)),
	)
	defer span.End()
	if rec.ID == "" {
		err := errors.New("bundle not found")
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	defer func() {
		_, _ = s.state.ReleaseInflight(ctx, rec.ClientID)
	}()
	rec, err := s.state.TransitionBundle(ctx, rec.ID, terminal, model.StatePersisted, reason)
	if err != nil {
		s.metrics.IncTerminalError()
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	if err := s.emitTransition(ctx, rec, terminal, model.StatePersisted, "persisted"); err != nil {
		s.metrics.IncBrokerError()
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	rec, err = s.state.TransitionBundle(ctx, rec.ID, model.StatePersisted, model.StateCompleted, "completed")
	if err != nil {
		s.metrics.IncTerminalError()
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	if err := s.emitTransition(ctx, rec, model.StatePersisted, model.StateCompleted, "completed"); err != nil {
		s.metrics.IncBrokerError()
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	events, err := s.state.ListEvents(ctx, rec.ID, s.cfg.HistoryLimit)
	if err != nil {
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	leaves := make([][]byte, 0, len(events))
	for _, ev := range events {
		body, _ := json.Marshal(ev)
		leaves = append(leaves, body)
	}
	root := commitment.Root(leaves...)
	auth := s.currentAuthority()
	cp := model.CheckpointRecord{
		BatchID:    checkpointID(rec.ID, rec.Sequence),
		BundleID:   rec.ID,
		ShardID:    rec.ShardID,
		ObjectKey:  checkpointObjectKey(rec.ShardID, rec.ID, rec.Sequence),
		Epoch:      auth.Epoch,
		Root:       hex.EncodeToString(root[:]),
		EventCount: len(events),
		LastOffset: rec.Sequence,
		RegionID:   rec.RegionID,
		SignedBy:   "relay",
		Signature:  hex.EncodeToString(signature(root, rec, auth)),
		Time:       nowUTC(),
		Version:    rec.Version,
	}
	cpBody, err := json.Marshal(cp)
	if err != nil {
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	start := time.Now()
	if _, err := s.checkpts.Put(ctx, cp, cpBody); err != nil {
		s.metrics.IncCheckpointError()
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	s.metrics.ObserveCheckpointLatency(time.Since(start))
	start = time.Now()
	if err := s.state.PutCheckpoint(ctx, cp); err != nil {
		s.metrics.IncStateError()
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	s.metrics.ObserveStateLatency(time.Since(start))
	start = time.Now()
	if err := s.wal.AppendEncoded(ctx, "checkpoint", cpBody); err != nil {
		s.metrics.IncWALError()
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	s.metrics.ObserveWALLatency(time.Since(start))
	start = time.Now()
	if err := s.broker.Publish(ctx, broker.Message{
		Topic:     s.cfg.BrokerTopic + ".checkpoint",
		Key:       rec.ID,
		Sequence:  uint64(cp.Version),
		Timestamp: cp.Time,
		Headers:   map[string]string{"kind": "checkpoint"},
		Payload:   cpBody,
	}); err != nil {
		s.metrics.IncBrokerError()
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	s.metrics.ObserveBrokerLatency(time.Since(start))
	if err := s.state.DeleteEvents(ctx, rec.ID); err != nil {
		s.metrics.IncStateError()
		endSpan(span, err)
		return model.BundleRecord{}, err
	}
	switch terminal {
	case model.StateRejected:
		s.metrics.IncRejected()
	case model.StateDeadLetter:
		s.metrics.IncDeadLetter()
	case model.StateForwarded:
		s.metrics.IncForwarded()
	}
	endSpan(span, nil)
	return rec, nil
}

func signature(root [32]byte, rec model.BundleRecord, auth graph.Authority) []byte {
	h := sha256.New()
	h.Write(root[:])
	h.Write([]byte(rec.ID))
	h.Write([]byte(rec.RegionID))
	h.Write([]byte(rec.ShardID))
	h.Write([]byte(auth.LeaseID))
	h.Write([]byte(auth.ShardID))
	h.Write([]byte(fmt.Sprintf("%d:%d", auth.Epoch, auth.FenceToken)))
	return h.Sum(nil)
}

func nowUTC() time.Time { return time.Now().UTC() }
