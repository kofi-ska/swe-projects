package valkey

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"mevrelayv3/internal/graph"
	"mevrelayv3/internal/lifecycle"
	"mevrelayv3/internal/model"
	state "mevrelayv3/internal/state"
)

type Store struct {
	client    *redis.Client
	prefix    string
	shardID   string
	retention time.Duration
	history   int
}

func New(url string, retention time.Duration, history int, shardID string) (*Store, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	if retention <= 0 {
		retention = 24 * time.Hour
	}
	if history <= 0 {
		history = 256
	}
	return &Store{
		client:    redis.NewClient(opt),
		prefix:    "mev:v3",
		shardID:   shardID,
		retention: retention,
		history:   history,
	}, nil
}

func (s *Store) key(parts ...string) string {
	return s.prefix + ":" + strings.Join(parts, ":")
}

func (s *Store) CreateBundle(ctx context.Context, rec model.BundleRecord) (model.BundleRecord, error) {
	hashKey := s.key("hash", rec.BundleHash)
	bundleKey := s.key("bundle", rec.ID)
	bundlesKey := s.key("bundles")
	authKey := s.authorityKey(rec.ShardID)
	var duplicateID string
	for {
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			if existing, err := tx.Get(ctx, hashKey).Result(); err == nil && existing != "" {
				duplicateID = existing
				return state.ErrDuplicateBundle
			}
			if rec.ShardID != "" {
				m, err := tx.HGetAll(ctx, authKey).Result()
				if err != nil {
					return err
				}
				auth, ok, err := authorityFromMap(m)
				if err != nil {
					return err
				}
				if !ok || validateAuthority(rec, auth) != nil {
					return state.ErrStaleAuthority
				}
			}
			_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, hashKey, rec.ID, s.retention)
				pipe.HSet(ctx, bundleKey, recordToMap(rec))
				pipe.ZAdd(ctx, bundlesKey, redis.Z{
					Score:  float64(rec.CreatedAt.UTC().UnixNano()),
					Member: rec.ID,
				})
				pipe.Expire(ctx, bundleKey, s.retention)
				pipe.Expire(ctx, bundlesKey, s.retention)
				pipe.Expire(ctx, hashKey, s.retention)
				return nil
			})
			return err
		}, hashKey, bundleKey, bundlesKey, authKey)
		if err == redis.TxFailedErr {
			continue
		}
		if err != nil {
			if errors.Is(err, state.ErrDuplicateBundle) {
				if duplicateID != "" {
					if existing, ok, loadErr := s.GetBundle(ctx, duplicateID); ok && loadErr == nil {
						return existing, state.ErrDuplicateBundle
					}
				}
				return model.BundleRecord{}, state.ErrDuplicateBundle
			}
			return model.BundleRecord{}, err
		}
		return rec, nil
	}
}

func (s *Store) GetBundle(ctx context.Context, id string) (model.BundleRecord, bool, error) {
	m, err := s.client.HGetAll(ctx, s.key("bundle", id)).Result()
	if err != nil {
		return model.BundleRecord{}, false, err
	}
	if len(m) == 0 {
		return model.BundleRecord{}, false, nil
	}
	return mapToRecord(m)
}

func (s *Store) ListBundles(ctx context.Context, limit int) ([]model.BundleRecord, error) {
	if limit <= 0 || limit > s.history {
		limit = s.history
	}
	ids, err := s.client.ZRevRange(ctx, s.key("bundles"), 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]model.BundleRecord, 0, len(ids))
	for _, id := range ids {
		rec, ok, err := s.GetBundle(ctx, id)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, rec)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *Store) TransitionBundle(ctx context.Context, id string, from, to model.BundleState, reason string) (model.BundleRecord, error) {
	if err := lifecycle.ValidateTransition(from, to); err != nil {
		return model.BundleRecord{}, err
	}
	key := s.key("bundle", id)
	authKey := s.authorityKey(s.shardID)
	for {
		var next model.BundleRecord
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			m, err := tx.HGetAll(ctx, key).Result()
			if err != nil {
				return err
			}
			if len(m) == 0 {
				return state.ErrBundleNotFound
			}
			rec, ok, err := mapToRecord(m)
			if err != nil || !ok {
				return err
			}
			if rec.State != from {
				return state.ErrStateMismatch
			}
			auth, ok, err := s.currentAuthorityTx(ctx, tx, rec.ShardID)
			if err != nil {
				return err
			}
			if !ok || validateAuthority(rec, auth) != nil {
				return state.ErrStaleAuthority
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
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.HSet(ctx, key, recordToMap(rec))
				pipe.Expire(ctx, key, s.retention)
				pipe.Expire(ctx, s.key("hash", rec.BundleHash), s.retention)
				pipe.Expire(ctx, s.key("bundles"), s.retention)
				pipe.Expire(ctx, authKey, s.retention)
				return nil
			})
			if err != nil {
				return err
			}
			next = rec
			return nil
		}, key, authKey)
		if err == redis.TxFailedErr {
			continue
		}
		if err != nil {
			return model.BundleRecord{}, err
		}
		return next, nil
	}
}

func (s *Store) UpdateRetryCount(ctx context.Context, id string, retryCount int) (model.BundleRecord, error) {
	return s.updateBundle(ctx, id, func(rec *model.BundleRecord) {
		rec.RetryCount = retryCount
	})
}

func (s *Store) UpdateResult(ctx context.Context, id string, score, profit float64, reason string) (model.BundleRecord, error) {
	return s.updateBundle(ctx, id, func(rec *model.BundleRecord) {
		rec.Score = score
		rec.ProfitEth = profit
		rec.Reason = reason
	})
}

func (s *Store) updateBundle(ctx context.Context, id string, fn func(*model.BundleRecord)) (model.BundleRecord, error) {
	key := s.key("bundle", id)
	authKey := s.authorityKey(s.shardID)
	for {
		var next model.BundleRecord
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			m, err := tx.HGetAll(ctx, key).Result()
			if err != nil {
				return err
			}
			if len(m) == 0 {
				return state.ErrBundleNotFound
			}
			rec, ok, err := mapToRecord(m)
			if err != nil || !ok {
				return err
			}
			auth, ok, err := s.currentAuthorityTx(ctx, tx, rec.ShardID)
			if err != nil {
				return err
			}
			if !ok || validateAuthority(rec, auth) != nil {
				return state.ErrStaleAuthority
			}
			fn(&rec)
			rec.Version++
			rec.UpdatedAt = time.Now().UTC()
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.HSet(ctx, key, recordToMap(rec))
				pipe.Expire(ctx, key, s.retention)
				pipe.Expire(ctx, s.key("hash", rec.BundleHash), s.retention)
				pipe.Expire(ctx, s.key("bundles"), s.retention)
				pipe.Expire(ctx, authKey, s.retention)
				return nil
			})
			if err != nil {
				return err
			}
			next = rec
			return nil
		}, key, authKey)
		if err == redis.TxFailedErr {
			continue
		}
		if err != nil {
			return model.BundleRecord{}, err
		}
		return next, nil
	}
}

func (s *Store) ReserveInflight(ctx context.Context, clientID string, limit int) (int, error) {
	key := s.key("client", clientID, "inflight")
	script := redis.NewScript(`
local current = tonumber(redis.call("GET", KEYS[1]) or "0")
local limit = tonumber(ARGV[1])
if current >= limit then
  return -1
end
current = current + 1
redis.call("SET", KEYS[1], current)
return current
`)
	res, err := script.Run(ctx, s.client, []string{key}, limit).Int64()
	if err != nil {
		return 0, err
	}
	if res < 0 {
		cur, _ := s.GetInflight(ctx, clientID)
		return cur, state.ErrClientInflight
	}
	_ = s.touch(ctx, key)
	return int(res), nil
}

func (s *Store) ReleaseInflight(ctx context.Context, clientID string) (int, error) {
	key := s.key("client", clientID, "inflight")
	script := redis.NewScript(`
local current = tonumber(redis.call("GET", KEYS[1]) or "0")
if current <= 0 then
  redis.call("SET", KEYS[1], 0)
  return 0
end
current = current - 1
redis.call("SET", KEYS[1], current)
return current
`)
	res, err := script.Run(ctx, s.client, []string{key}).Int64()
	_ = s.touch(ctx, key)
	return int(res), err
}

func (s *Store) GetInflight(ctx context.Context, clientID string) (int, error) {
	n, err := s.client.Get(ctx, s.key("client", clientID, "inflight")).Int()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return n, err
}

func (s *Store) ScheduleRetry(ctx context.Context, id string, due time.Time) error {
	if err := s.client.ZAdd(ctx, s.key("retries"), redis.Z{
		Score:  float64(due.UTC().UnixNano()),
		Member: id,
	}).Err(); err != nil {
		return err
	}
	return s.touch(ctx, s.key("retries"))
}

func (s *Store) ClaimDueRetries(ctx context.Context, now time.Time, limit int) ([]string, error) {
	script := redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local ids = redis.call("ZRANGEBYSCORE", key, "-inf", now, "LIMIT", 0, limit)
if #ids > 0 then
  redis.call("ZREM", key, unpack(ids))
end
return ids
`)
	raw, err := script.Run(ctx, s.client, []string{s.key("retries")}, now.UTC().UnixNano(), limit).Result()
	if err != nil {
		return nil, err
	}
	ids, ok := raw.([]interface{})
	if !ok {
		return nil, errors.New("unexpected retry claim response")
	}
	out := make([]string, 0, len(ids))
	for _, v := range ids {
		switch x := v.(type) {
		case string:
			out = append(out, x)
		case []byte:
			out = append(out, string(x))
		}
	}
	return out, nil
}

func (s *Store) AppendEvent(ctx context.Context, ev model.EventRecord) error {
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	key := s.key("events", ev.BundleID)
	if err := s.client.RPush(ctx, key, body).Err(); err != nil {
		return err
	}
	if err := s.client.LTrim(ctx, key, int64(-s.history), -1).Err(); err != nil {
		return err
	}
	return s.touch(ctx, key)
}

func (s *Store) ListEvents(ctx context.Context, bundleID string, limit int) ([]model.EventRecord, error) {
	if limit <= 0 || limit > s.history {
		limit = s.history
	}
	vals, err := s.client.LRange(ctx, s.key("events", bundleID), int64(-limit), -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]model.EventRecord, 0, len(vals))
	for _, raw := range vals {
		var ev model.EventRecord
		if err := json.Unmarshal([]byte(raw), &ev); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, nil
}

func (s *Store) PutCheckpoint(ctx context.Context, cp model.CheckpointRecord) error {
	body, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	if err := s.client.Set(ctx, s.key("checkpoint", cp.BatchID), body, 0).Err(); err != nil {
		return err
	}
	if err := s.client.ZAdd(ctx, s.key("checkpoints"), redis.Z{
		Score:  float64(cp.Time.UTC().UnixNano()),
		Member: cp.BatchID,
	}).Err(); err != nil {
		return err
	}
	return s.touch(ctx, s.key("checkpoint", cp.BatchID), s.key("checkpoints"))
}

func (s *Store) ListCheckpoints(ctx context.Context, limit int) ([]model.CheckpointRecord, error) {
	if limit <= 0 || limit > s.history {
		limit = s.history
	}
	ids, err := s.client.ZRevRange(ctx, s.key("checkpoints"), 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]model.CheckpointRecord, 0, len(ids))
	for _, id := range ids {
		raw, err := s.client.Get(ctx, s.key("checkpoint", id)).Bytes()
		if errors.Is(err, redis.Nil) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var cp model.CheckpointRecord
		if err := json.Unmarshal(raw, &cp); err != nil {
			return nil, err
		}
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out, nil
}

func (s *Store) DeleteEvents(ctx context.Context, bundleID string) error {
	return s.client.Del(ctx, s.key("events", bundleID)).Err()
}

func (s *Store) SetAuthority(ctx context.Context, auth graph.Authority) error {
	key := s.key("authority", auth.ShardID)
	fields := authorityToMap(auth)
	if err := s.client.HSet(ctx, key, fields).Err(); err != nil {
		return err
	}
	ttl := time.Until(auth.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Second
	}
	if err := s.client.Expire(ctx, key, ttl).Err(); err != nil {
		return err
	}
	return nil
}

func (s *Store) GetAuthority(ctx context.Context, shardID string) (graph.Authority, bool, error) {
	m, err := s.client.HGetAll(ctx, s.key("authority", shardID)).Result()
	if err != nil {
		return graph.Authority{}, false, err
	}
	if len(m) == 0 {
		return graph.Authority{}, false, nil
	}
	auth, ok, err := authorityFromMap(m)
	return auth, ok, err
}

func (s *Store) Health(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *Store) Close() error {
	return s.client.Close()
}

func (s *Store) currentAuthorityTx(ctx context.Context, tx *redis.Tx, shardID string) (graph.Authority, bool, error) {
	m, err := tx.HGetAll(ctx, s.key("authority", shardID)).Result()
	if err != nil {
		return graph.Authority{}, false, err
	}
	if len(m) == 0 {
		return graph.Authority{}, false, nil
	}
	auth, ok, err := authorityFromMap(m)
	if err != nil {
		return graph.Authority{}, false, err
	}
	return auth, ok, nil
}

func (s *Store) touch(ctx context.Context, keys ...string) error {
	if s.retention <= 0 {
		return nil
	}
	for _, key := range keys {
		if err := s.client.Expire(ctx, key, s.retention).Err(); err != nil {
			return err
		}
	}
	return nil
}

func recordToMap(rec model.BundleRecord) map[string]interface{} {
	return map[string]interface{}{
		"id":                  rec.ID,
		"bundle_hash":         rec.BundleHash,
		"request":             mustJSON(rec.Request),
		"client_id":           rec.ClientID,
		"region_id":           rec.RegionID,
		"shard_id":            rec.ShardID,
		"lease_id":            rec.LeaseID,
		"lease_epoch":         strconv.FormatUint(rec.LeaseEpoch, 10),
		"fence_token":         strconv.FormatUint(rec.FenceToken, 10),
		"state":               string(rec.State),
		"retry_count":         strconv.Itoa(rec.RetryCount),
		"score":               strconv.FormatFloat(rec.Score, 'f', -1, 64),
		"profit_eth":          strconv.FormatFloat(rec.ProfitEth, 'f', -1, 64),
		"reason":              rec.Reason,
		"terminal":            rec.Terminal,
		"version":             strconv.FormatInt(rec.Version, 10),
		"sequence":            strconv.FormatUint(rec.Sequence, 10),
		"created_at":          rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":          rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
		"queued_at":           rec.QueuedAt.UTC().Format(time.RFC3339Nano),
		"completed_at":        rec.CompletedAt.UTC().Format(time.RFC3339Nano),
		"deadline_at":         rec.DeadlineAt.UTC().Format(time.RFC3339Nano),
		"expected_value":      strconv.FormatFloat(rec.ExpectedValue, 'f', -1, 64),
		"expected_cost":       strconv.FormatFloat(rec.ExpectedCost, 'f', -1, 64),
		"expected_service_ms": strconv.FormatInt(rec.ExpectedServiceMS, 10),
		"priority":            strconv.FormatFloat(rec.Priority, 'f', -1, 64),
	}
}

func mapToRecord(m map[string]string) (model.BundleRecord, bool, error) {
	if len(m) == 0 {
		return model.BundleRecord{}, false, nil
	}
	var rec model.BundleRecord
	rec.ID = m["id"]
	rec.BundleHash = m["bundle_hash"]
	rec.ClientID = m["client_id"]
	rec.RegionID = m["region_id"]
	rec.ShardID = m["shard_id"]
	rec.LeaseID = m["lease_id"]
	rec.LeaseEpoch = mustUint64(m["lease_epoch"])
	rec.FenceToken = mustUint64(m["fence_token"])
	rec.State = model.BundleState(m["state"])
	rec.RetryCount = mustInt(m["retry_count"])
	rec.Score = mustFloat(m["score"])
	rec.ProfitEth = mustFloat(m["profit_eth"])
	rec.Reason = m["reason"]
	rec.Terminal = m["terminal"]
	rec.Version = mustInt64(m["version"])
	rec.Sequence = uint64(mustInt64(m["sequence"]))
	rec.CreatedAt = mustTime(m["created_at"])
	rec.UpdatedAt = mustTime(m["updated_at"])
	rec.QueuedAt = mustTime(m["queued_at"])
	rec.CompletedAt = mustTime(m["completed_at"])
	rec.DeadlineAt = mustTime(m["deadline_at"])
	rec.ExpectedValue = mustFloat(m["expected_value"])
	rec.ExpectedCost = mustFloat(m["expected_cost"])
	rec.ExpectedServiceMS = mustInt64(m["expected_service_ms"])
	rec.Priority = mustFloat(m["priority"])
	if raw := m["request"]; raw != "" {
		if err := json.Unmarshal([]byte(raw), &rec.Request); err != nil {
			return model.BundleRecord{}, false, err
		}
	}
	return rec, true, nil
}

func authorityToMap(auth graph.Authority) map[string]interface{} {
	return map[string]interface{}{
		"shard_id":    auth.ShardID,
		"lease_id":    auth.LeaseID,
		"epoch":       strconv.FormatUint(auth.Epoch, 10),
		"fence_token": strconv.FormatUint(auth.FenceToken, 10),
		"issued_at":   auth.IssuedAt.UTC().Format(time.RFC3339Nano),
		"expires_at":  auth.ExpiresAt.UTC().Format(time.RFC3339Nano),
	}
}

func authorityFromMap(m map[string]string) (graph.Authority, bool, error) {
	if len(m) == 0 {
		return graph.Authority{}, false, nil
	}
	auth := graph.Authority{
		ShardID:    m["shard_id"],
		LeaseID:    m["lease_id"],
		Epoch:      mustUint64(m["epoch"]),
		FenceToken: mustUint64(m["fence_token"]),
		IssuedAt:   mustTime(m["issued_at"]),
		ExpiresAt:  mustTime(m["expires_at"]),
	}
	return auth, true, nil
}

func validateAuthority(rec model.BundleRecord, auth graph.Authority) error {
	if rec.ShardID == "" {
		return nil
	}
	now := time.Now().UTC()
	if !auth.Valid(now) {
		return state.ErrStaleAuthority
	}
	if auth.ShardID != rec.ShardID {
		return state.ErrStaleAuthority
	}
	if rec.LeaseID != "" && rec.LeaseID != auth.LeaseID {
		return state.ErrStaleAuthority
	}
	if rec.LeaseEpoch != 0 && rec.LeaseEpoch != auth.Epoch {
		return state.ErrStaleAuthority
	}
	if rec.FenceToken != 0 && rec.FenceToken != auth.FenceToken {
		return state.ErrStaleAuthority
	}
	return nil
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func mustInt(v string) int {
	n, _ := strconv.Atoi(v)
	return n
}

func mustInt64(v string) int64 {
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

func mustUint64(v string) uint64 {
	n, _ := strconv.ParseUint(v, 10, 64)
	return n
}

func mustFloat(v string) float64 {
	n, _ := strconv.ParseFloat(v, 64)
	return n
}

func mustTime(v string) time.Time {
	if v == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339Nano, v)
	return t
}

func (s *Store) authorityKey(shardID string) string {
	if shardID == "" {
		shardID = s.shardID
	}
	return s.key("authority", shardID)
}
