package minio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"mevrelayv3/internal/checkpoint"
	"mevrelayv3/internal/model"
)

type Store struct {
	mu     sync.Mutex
	client *minio.Client
	bucket string
	prefix string
	closed bool
}

func New(endpoint, accessKey, secretKey, bucket string, useSSL bool, prefix string) (*Store, error) {
	if endpoint == "" {
		return nil, errors.New("empty minio endpoint")
	}
	if bucket == "" {
		return nil, errors.New("empty minio bucket")
	}
	if prefix == "" {
		prefix = "checkpoints"
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, err
	}
	s := &Store{
		client: client,
		bucket: bucket,
		prefix: prefix,
	}
	if err := s.ensureBucket(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Put(ctx context.Context, cp model.CheckpointRecord, body []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return "", errors.New("checkpoint store closed")
	}
	key := cp.ObjectKey
	if key == "" {
		key = s.key(cp.ShardID, cp.BatchID)
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(body), int64(len(body)), minio.PutObjectOptions{
		ContentType: "application/json",
	})
	if err != nil {
		return "", err
	}
	return key, nil
}

func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New("checkpoint store closed")
	}
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	body := new(bytes.Buffer)
	if _, err := body.ReadFrom(obj); err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}

func (s *Store) Health(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("checkpoint store closed")
	}
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("bucket %s missing", s.bucket)
	}
	return nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *Store) ensureBucket(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	return s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
}

func (s *Store) key(shardID, batchID string) string {
	if shardID == "" {
		return s.prefix + "/" + batchID + ".json"
	}
	return s.prefix + "/" + shardID + "/" + batchID + ".json"
}

var _ checkpoint.Store = (*Store)(nil)
