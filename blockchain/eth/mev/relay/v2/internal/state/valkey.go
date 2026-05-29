package state

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"mevrelayv2/internal/lifecycle"
	"mevrelayv2/internal/model"
)

// ValkeyStore owns authoritative v2 coordination state in Valkey.
type ValkeyStore struct {
	client    *redis.Client
	prefix    string
	retention time.Duration
	history   int
}

// NewValkey creates a Valkey-backed store.
func NewValkey(url string, retention time.Duration, history int) (*ValkeyStore, error) {
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
	return &ValkeyStore{
		client:    redis.NewClient(opt),
		prefix:    "mev:v2",
		retention: retention,
		history:   history,
	}, nil
}

func (s *ValkeyStore) key(parts ...string) string {
	return s.prefix + ":" + strings.Join(parts, ":")
}

func (s *ValkeyStore) CreateBundle(ctx context.Context, rec model.BundleRecord) (model.BundleRecord, error) {
	hashKey := s.key("hash", rec.BundleHash)
	bundleKey := s.key("bundle", rec.ID)
	args := s.createBundleArgs(rec)
	script := redis.NewScript(`
local hashKey = KEYS[1]
local bundleKey = KEYS[2]
local bundlesKey = KEYS[3]
local retention = tonumber(ARGV[1])
local createdAt = tonumber(ARGV[2])
local bundleID = ARGV[3]
local existing = redis.call("GET", hashKey)
if existing and existing ~= "" then
  return {0, existing}
end
redis.call("SET", hashKey, bundleID)
for i = 4, #ARGV, 2 do
  redis.call("HSET", bundleKey, ARGV[i], ARGV[i + 1])
end
redis.call("ZADD", bundlesKey, createdAt, bundleID)
if retention > 0 then
  redis.call("PEXPIRE", hashKey, retention)
  redis.call("PEXPIRE", bundleKey, retention)
  redis.call("PEXPIRE", bundlesKey, retention)
end
return {1, bundleID}
`)
	raw, err := script.Run(ctx, s.client, []string{hashKey, bundleKey, s.key("bundles")}, args...).Result()
	if err != nil {
		return model.BundleRecord{}, err
	}
	reply, ok := raw.([]interface{})
	if !ok || len(reply) < 2 {
		return model.BundleRecord{}, errors.New("unexpected create bundle response")
	}
	switch v := reply[0].(type) {
	case int64:
		if v == 0 {
			existing := toString(reply[1])
			if existing != "" {
				if rec2, ok, _ := s.GetBundle(ctx, existing); ok {
					return rec2, ErrDuplicateBundle
				}
			}
			return model.BundleRecord{}, ErrDuplicateBundle
		}
	case string:
		if v == "0" {
			existing := toString(reply[1])
			if existing != "" {
				if rec2, ok, _ := s.GetBundle(ctx, existing); ok {
					return rec2, ErrDuplicateBundle
				}
			}
			return model.BundleRecord{}, ErrDuplicateBundle
		}
	}
	return rec, nil
}

func (s *ValkeyStore) GetBundle(ctx context.Context, id string) (model.BundleRecord, bool, error) {
	m, err := s.client.HGetAll(ctx, s.key("bundle", id)).Result()
	if err != nil {
		return model.BundleRecord{}, false, err
	}
	if len(m) == 0 {
		return model.BundleRecord{}, false, nil
	}
	return mapToRecord(m)
}

func (s *ValkeyStore) ListBundles(ctx context.Context, limit int) ([]model.BundleRecord, error) {
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

func (s *ValkeyStore) TransitionBundle(ctx context.Context, id string, from, to model.BundleState, reason string) (model.BundleRecord, error) {
	if err := lifecycle.ValidateTransition(from, to); err != nil {
		return model.BundleRecord{}, err
	}
	key := s.key("bundle", id)
	for {
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			m, err := tx.HGetAll(ctx, key).Result()
			if err != nil {
				return err
			}
			if len(m) == 0 {
				return ErrBundleNotFound
			}
			rec, ok, err := mapToRecord(m)
			if err != nil || !ok {
				return err
			}
			if rec.State != from {
				return ErrStateMismatch
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
				return nil
			})
			return err
		}, key)
		if err == redis.TxFailedErr {
			continue
		}
		if err != nil {
			return model.BundleRecord{}, err
		}
		rec, _, err := s.GetBundle(ctx, id)
		return rec, err
	}
}

func (s *ValkeyStore) UpdateRetryCount(ctx context.Context, id string, retryCount int) (model.BundleRecord, error) {
	return s.updateBundle(ctx, id, func(rec *model.BundleRecord) {
		rec.RetryCount = retryCount
	})
}

func (s *ValkeyStore) UpdateResult(ctx context.Context, id string, score, profit float64, reason string) (model.BundleRecord, error) {
	return s.updateBundle(ctx, id, func(rec *model.BundleRecord) {
		rec.Score = score
		rec.ProfitEth = profit
		rec.Reason = reason
	})
}

func (s *ValkeyStore) updateBundle(ctx context.Context, id string, fn func(*model.BundleRecord)) (model.BundleRecord, error) {
	key := s.key("bundle", id)
	for {
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			m, err := tx.HGetAll(ctx, key).Result()
			if err != nil {
				return err
			}
			if len(m) == 0 {
				return ErrBundleNotFound
			}
			rec, ok, err := mapToRecord(m)
			if err != nil || !ok {
				return err
			}
			fn(&rec)
			rec.Version++
			rec.UpdatedAt = time.Now().UTC()
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.HSet(ctx, key, recordToMap(rec))
				pipe.Expire(ctx, key, s.retention)
				pipe.Expire(ctx, s.key("hash", rec.BundleHash), s.retention)
				pipe.Expire(ctx, s.key("bundles"), s.retention)
				return nil
			})
			return err
		}, key)
		if err == redis.TxFailedErr {
			continue
		}
		if err != nil {
			return model.BundleRecord{}, err
		}
		rec, _, err := s.GetBundle(ctx, id)
		return rec, err
	}
}

func (s *ValkeyStore) ReserveInflight(ctx context.Context, clientID string, limit int) (int, error) {
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
		return cur, ErrClientInflight
	}
	_ = s.touch(ctx, key)
	return int(res), nil
}

func (s *ValkeyStore) ReleaseInflight(ctx context.Context, clientID string) (int, error) {
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

func (s *ValkeyStore) GetInflight(ctx context.Context, clientID string) (int, error) {
	n, err := s.client.Get(ctx, s.key("client", clientID, "inflight")).Int()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return n, err
}

func (s *ValkeyStore) ScheduleRetry(ctx context.Context, id string, due time.Time) error {
	if err := s.client.ZAdd(ctx, s.key("retries"), redis.Z{
		Score:  float64(due.UTC().UnixNano()),
		Member: id,
	}).Err(); err != nil {
		return err
	}
	return s.touch(ctx, s.key("retries"))
}

func (s *ValkeyStore) ClaimDueRetries(ctx context.Context, now time.Time, limit int) ([]string, error) {
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

func (s *ValkeyStore) AppendEvent(ctx context.Context, ev model.EventRecord) error {
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

func (s *ValkeyStore) ListEvents(ctx context.Context, bundleID string, limit int) ([]model.EventRecord, error) {
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

func (s *ValkeyStore) PutCheckpoint(ctx context.Context, cp model.CheckpointRecord) error {
	body, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	if err := s.client.Set(ctx, s.key("checkpoint", cp.BatchID), body, 0).Err(); err != nil {
		return err
	}
	if err := s.client.ZAdd(ctx, s.key("checkpoints"), redis.Z{Score: float64(cp.Time.UTC().UnixNano()), Member: cp.BatchID}).Err(); err != nil {
		return err
	}
	return s.touch(ctx, s.key("checkpoint", cp.BatchID), s.key("checkpoints"))
}

func (s *ValkeyStore) ListCheckpoints(ctx context.Context, limit int) ([]model.CheckpointRecord, error) {
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

func (s *ValkeyStore) DeleteEvents(ctx context.Context, bundleID string) error {
	return s.client.Del(ctx, s.key("events", bundleID)).Err()
}

func (s *ValkeyStore) Health(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *ValkeyStore) Close() error {
	return s.client.Close()
}

func (s *ValkeyStore) touch(ctx context.Context, keys ...string) error {
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
		"state":               string(rec.State),
		"retry_count":         rec.RetryCount,
		"score":               rec.Score,
		"profit_eth":          rec.ProfitEth,
		"reason":              rec.Reason,
		"terminal":            rec.Terminal,
		"version":             rec.Version,
		"sequence":            rec.Sequence,
		"created_at":          rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":          rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
		"queued_at":           rec.QueuedAt.UTC().Format(time.RFC3339Nano),
		"completed_at":        rec.CompletedAt.UTC().Format(time.RFC3339Nano),
		"deadline_at":         rec.DeadlineAt.UTC().Format(time.RFC3339Nano),
		"expected_value":      rec.ExpectedValue,
		"expected_cost":       rec.ExpectedCost,
		"expected_service_ms": rec.ExpectedServiceMS,
		"priority":            rec.Priority,
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

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func (s *ValkeyStore) createBundleArgs(rec model.BundleRecord) []any {
	fields := recordFieldPairs(rec)
	args := make([]any, 0, 3+len(fields)+2)
	args = append(args, s.retention.Milliseconds(), rec.CreatedAt.UTC().UnixNano(), rec.ID)
	for _, pair := range fields {
		args = append(args, pair[0], pair[1])
	}
	return args
}

func recordFieldPairs(rec model.BundleRecord) [][2]string {
	return [][2]string{
		{"id", rec.ID},
		{"bundle_hash", rec.BundleHash},
		{"request", mustJSON(rec.Request)},
		{"client_id", rec.ClientID},
		{"region_id", rec.RegionID},
		{"state", string(rec.State)},
		{"retry_count", strconv.Itoa(rec.RetryCount)},
		{"score", strconv.FormatFloat(rec.Score, 'f', -1, 64)},
		{"profit_eth", strconv.FormatFloat(rec.ProfitEth, 'f', -1, 64)},
		{"reason", rec.Reason},
		{"terminal", rec.Terminal},
		{"version", strconv.FormatInt(rec.Version, 10)},
		{"sequence", strconv.FormatUint(rec.Sequence, 10)},
		{"created_at", rec.CreatedAt.UTC().Format(time.RFC3339Nano)},
		{"updated_at", rec.UpdatedAt.UTC().Format(time.RFC3339Nano)},
		{"queued_at", rec.QueuedAt.UTC().Format(time.RFC3339Nano)},
		{"completed_at", rec.CompletedAt.UTC().Format(time.RFC3339Nano)},
		{"deadline_at", rec.DeadlineAt.UTC().Format(time.RFC3339Nano)},
		{"expected_value", strconv.FormatFloat(rec.ExpectedValue, 'f', -1, 64)},
		{"expected_cost", strconv.FormatFloat(rec.ExpectedCost, 'f', -1, 64)},
		{"expected_service_ms", strconv.FormatInt(rec.ExpectedServiceMS, 10)},
		{"priority", strconv.FormatFloat(rec.Priority, 'f', -1, 64)},
	}
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	default:
		return ""
	}
}

func mustInt(v string) int {
	n, _ := strconv.Atoi(v)
	return n
}

func mustInt64(v string) int64 {
	n, _ := strconv.ParseInt(v, 10, 64)
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
