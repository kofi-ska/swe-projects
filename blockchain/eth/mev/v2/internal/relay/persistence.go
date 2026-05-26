package relay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"mevrelayv2/internal/broker"
	"mevrelayv2/internal/commitment"
	"mevrelayv2/internal/model"
)

func (s *Service) emitTransition(ctx context.Context, rec model.BundleRecord, from, to model.BundleState, reason string) error {
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
	}
	start := time.Now()
	if err := s.state.AppendEvent(ctx, ev); err != nil {
		s.metrics.IncStateError()
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
		return err
	}
	s.metrics.ObserveWALLatency(time.Since(start))
	start = time.Now()
	err = s.broker.Publish(ctx, broker.Message{
		Topic:     s.cfg.BrokerTopic,
		Key:       rec.ID,
		Sequence:  ev.Sequence,
		Timestamp: ev.Time,
		Headers:   map[string]string{"kind": "event"},
		Payload:   body,
	})
	s.metrics.ObserveBrokerLatency(time.Since(start))
	return err
}

func (s *Service) finishTerminal(ctx context.Context, rec model.BundleRecord, terminal model.BundleState, reason string) (model.BundleRecord, error) {
	if rec.ID == "" {
		return model.BundleRecord{}, errors.New("bundle not found")
	}
	defer func() {
		_, _ = s.state.ReleaseInflight(ctx, rec.ClientID)
	}()
	rec, err := s.state.TransitionBundle(ctx, rec.ID, terminal, model.StatePersisted, reason)
	if err != nil {
		s.metrics.IncTerminalError()
		return model.BundleRecord{}, err
	}
	if err := s.emitTransition(ctx, rec, terminal, model.StatePersisted, "persisted"); err != nil {
		s.metrics.IncBrokerError()
		return model.BundleRecord{}, err
	}
	rec, err = s.state.TransitionBundle(ctx, rec.ID, model.StatePersisted, model.StateCompleted, "completed")
	if err != nil {
		s.metrics.IncTerminalError()
		return model.BundleRecord{}, err
	}
	if err := s.emitTransition(ctx, rec, model.StatePersisted, model.StateCompleted, "completed"); err != nil {
		s.metrics.IncBrokerError()
		return model.BundleRecord{}, err
	}

	events, err := s.state.ListEvents(ctx, rec.ID, s.cfg.HistoryLimit)
	if err != nil {
		return model.BundleRecord{}, err
	}
	leaves := make([][]byte, 0, len(events))
	for _, ev := range events {
		body, _ := json.Marshal(ev)
		leaves = append(leaves, body)
	}
	root := commitment.Root(leaves...)
	cp := model.CheckpointRecord{
		BatchID:    checkpointID(rec.ID, rec.Sequence),
		BundleID:   rec.ID,
		Root:       hex.EncodeToString(root[:]),
		EventCount: len(events),
		RegionID:   rec.RegionID,
		SignedBy:   "relay",
		Signature:  hex.EncodeToString(signature(root, rec)),
		Time:       nowUTC(),
		Version:    rec.Version,
	}
	start := time.Now()
	if err := s.state.PutCheckpoint(ctx, cp); err != nil {
		s.metrics.IncStateError()
		return model.BundleRecord{}, err
	}
	s.metrics.ObserveStateLatency(time.Since(start))
	body, err := json.Marshal(cp)
	if err != nil {
		return model.BundleRecord{}, err
	}
	start = time.Now()
	if err := s.wal.AppendEncoded(ctx, "checkpoint", body); err != nil {
		s.metrics.IncWALError()
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
		Payload:   body,
	}); err != nil {
		s.metrics.IncBrokerError()
		return model.BundleRecord{}, err
	}
	s.metrics.ObserveBrokerLatency(time.Since(start))
	if err := s.state.DeleteEvents(ctx, rec.ID); err != nil {
		s.metrics.IncStateError()
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
	return rec, nil
}

func signature(root [32]byte, rec model.BundleRecord) []byte {
	h := sha256.New()
	h.Write(root[:])
	h.Write([]byte(rec.ID))
	h.Write([]byte(rec.RegionID))
	return h.Sum(nil)
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
