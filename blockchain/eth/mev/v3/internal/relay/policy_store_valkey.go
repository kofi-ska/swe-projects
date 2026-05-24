package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type policyValkeyStore struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

func newPolicyValkeyStore(url string, retention time.Duration) (*policyValkeyStore, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return &policyValkeyStore{
		client: redis.NewClient(opt),
		prefix: "mev:v3:policy",
		ttl:    policyTTL(retention),
	}, nil
}

func (s *policyValkeyStore) key(shardID string) string {
	return fmt.Sprintf("%s:%s", s.prefix, shardID)
}

func (s *policyValkeyStore) Load(ctx context.Context, shardID string) (PolicySnapshot, bool, error) {
	raw, err := s.client.Get(ctx, s.key(shardID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return PolicySnapshot{}, false, nil
	}
	if err != nil {
		return PolicySnapshot{}, false, err
	}
	var snap PolicySnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return PolicySnapshot{}, false, err
	}
	return snap, true, nil
}

func (s *policyValkeyStore) Save(ctx context.Context, shardID string, snap PolicySnapshot) error {
	body, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	if err := s.client.Set(ctx, s.key(shardID), body, s.ttl).Err(); err != nil {
		return err
	}
	return nil
}

func (s *policyValkeyStore) Health(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *policyValkeyStore) Close() error {
	return s.client.Close()
}

var _ PolicyStore = (*policyValkeyStore)(nil)
