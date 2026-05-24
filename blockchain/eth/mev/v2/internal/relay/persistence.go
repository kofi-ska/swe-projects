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
	if err := s.state.AppendEvent(ctx, ev); err != nil {
		return err
	}
	if err := s.wal.Append(ctx, "event", ev); err != nil {
		return err
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return s.broker.Publish(ctx, broker.Message{
		Topic:     s.cfg.BrokerTopic,
		Key:       rec.ID,
		Sequence:  ev.Sequence,
		Timestamp: ev.Time,
		Headers:   map[string]string{"kind": "event"},
		Payload:   body,
	})
}

func (s *Service) finishTerminal(ctx context.Context, id string, terminal model.BundleState, reason string) error {
	rec, ok, err := s.state.GetBundle(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("bundle not found")
	}
	defer func() {
		_, _ = s.state.ReleaseInflight(ctx, rec.ClientID)
	}()
	_, err = s.state.TransitionBundle(ctx, id, terminal, model.StatePersisted, reason)
	if err != nil {
		return err
	}
	rec, _, err = s.state.GetBundle(ctx, id)
	if err != nil {
		return err
	}
	if err := s.emitTransition(ctx, rec, terminal, model.StatePersisted, "persisted"); err != nil {
		return err
	}
	_, err = s.state.TransitionBundle(ctx, id, model.StatePersisted, model.StateCompleted, "completed")
	if err != nil {
		return err
	}
	rec, _, err = s.state.GetBundle(ctx, id)
	if err != nil {
		return err
	}
	if err := s.emitTransition(ctx, rec, model.StatePersisted, model.StateCompleted, "completed"); err != nil {
		return err
	}

	events, err := s.state.ListEvents(ctx, id, s.cfg.HistoryLimit)
	if err != nil {
		return err
	}
	leaves := make([][]byte, 0, len(events))
	for _, ev := range events {
		body, _ := json.Marshal(ev)
		leaves = append(leaves, body)
	}
	root := commitment.Root(leaves...)
	cp := model.CheckpointRecord{
		BatchID:    checkpointID(id, rec.Sequence),
		BundleID:   id,
		Root:       hex.EncodeToString(root[:]),
		EventCount: len(events),
		RegionID:   rec.RegionID,
		SignedBy:   "relay",
		Signature:  hex.EncodeToString(signature(root, rec)),
		Time:       nowUTC(),
		Version:    rec.Version,
	}
	if err := s.state.PutCheckpoint(ctx, cp); err != nil {
		return err
	}
	if err := s.wal.Append(ctx, "checkpoint", cp); err != nil {
		return err
	}
	body, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	if err := s.broker.Publish(ctx, broker.Message{
		Topic:     s.cfg.BrokerTopic + ".checkpoint",
		Key:       id,
		Sequence:  uint64(cp.Version),
		Timestamp: cp.Time,
		Headers:   map[string]string{"kind": "checkpoint"},
		Payload:   body,
	}); err != nil {
		return err
	}
	if err := s.state.DeleteEvents(ctx, id); err != nil {
		return err
	}
	return nil
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
