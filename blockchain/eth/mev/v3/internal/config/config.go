package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr             string
	DataDir              string
	RegionID             string
	NetworkID            string
	ShardID              string
	ShardSet             []string
	BackendKind          string
	StateKind            string
	ValkeyURL            string
	BrokerTopic          string
	BrokerKind           string
	BrokerURL            string
	BrokerBuffer         int
	CheckpointKind       string
	CheckpointEndpoint   string
	CheckpointAccessKey  string
	CheckpointSecretKey  string
	CheckpointBucket     string
	CheckpointUseSSL     bool
	QueueDepth           int
	WorkerCount          int
	MaxRetries           int
	RetryBackoff         time.Duration
	LeaseTTL             time.Duration
	LeaseRenewInterval   time.Duration
	RequestTimeout       time.Duration
	MaxPayloadBytes      int64
	MaxInFlightPerClient int
	HistoryLimit         int
	StateRetention       time.Duration
	WALMaxEntries        int
	AuditFlushEvery      int
	MaxQueueAge          time.Duration
	MinNetValue          float64
	MinDeadlineSlack     time.Duration
	ValuePerTx           float64
	CostPerTx            float64
	CostPerMS            float64
	OTELServiceName      string
	OTELExporterEndpoint string
	OTELExporterInsecure bool
	OTELSampleRatio      float64
	APIAuthToken         string
}

func Load() Config {
	shardID := env("SHARD_ID", "shard-0")
	shardSet := splitCSV(env("SHARD_SET", shardID))
	if len(shardSet) == 0 {
		shardSet = []string{shardID}
	}
	return Config{
		HTTPAddr:             env("HTTP_ADDR", ":8090"),
		DataDir:              env("DATA_DIR", "data"),
		RegionID:             env("REGION_ID", "local"),
		NetworkID:            env("NETWORK_ID", "mainnet"),
		ShardID:              shardID,
		ShardSet:             shardSet,
		BackendKind:          env("BACKEND_KIND", "local"),
		StateKind:            env("STATE_KIND", "valkey"),
		ValkeyURL:            env("VALKEY_URL", "redis://valkey:6379"),
		StateRetention:       envDuration("STATE_RETENTION", 24*time.Hour),
		BrokerTopic:          env("BROKER_TOPIC", "mevrelay.v3.events"),
		BrokerKind:           env("BROKER_KIND", "nats"),
		BrokerURL:            env("BROKER_URL", "nats://nats:4222"),
		BrokerBuffer:         envInt("BROKER_BUFFER", 256),
		CheckpointKind:       env("CHECKPOINT_KIND", "minio"),
		CheckpointEndpoint:   env("CHECKPOINT_ENDPOINT", "minio:9000"),
		CheckpointAccessKey:  env("CHECKPOINT_ACCESS_KEY", "minioadmin"),
		CheckpointSecretKey:  env("CHECKPOINT_SECRET_KEY", "minioadmin"),
		CheckpointBucket:     env("CHECKPOINT_BUCKET", "mevrelay-checkpoints"),
		CheckpointUseSSL:     envBool("CHECKPOINT_USE_SSL", false),
		QueueDepth:           envInt("QUEUE_DEPTH", 1024),
		WorkerCount:          envInt("WORKER_COUNT", 4),
		MaxRetries:           envInt("MAX_RETRIES", 3),
		RetryBackoff:         envDuration("RETRY_BACKOFF", 500*time.Millisecond),
		LeaseTTL:             envDuration("LEASE_TTL", 5*time.Second),
		LeaseRenewInterval:   envDuration("LEASE_RENEW_INTERVAL", 1*time.Second),
		RequestTimeout:       envDuration("REQUEST_TIMEOUT", 2*time.Second),
		MaxPayloadBytes:      int64(envInt("MAX_PAYLOAD_BYTES", 256*1024)),
		MaxInFlightPerClient: envInt("MAX_INFLIGHT_PER_CLIENT", 20),
		HistoryLimit:         envInt("HISTORY_LIMIT", 256),
		WALMaxEntries:        envInt("WAL_MAX_ENTRIES", 2048),
		AuditFlushEvery:      envInt("AUDIT_FLUSH_EVERY", 128),
		MaxQueueAge:          envDuration("MAX_QUEUE_AGE", 3*time.Second),
		MinNetValue:          envFloat("MIN_NET_VALUE", 0),
		MinDeadlineSlack:     envDuration("MIN_DEADLINE_SLACK", 500*time.Millisecond),
		ValuePerTx:           envFloat("VALUE_PER_TX", 1),
		CostPerTx:            envFloat("COST_PER_TX", 0.25),
		CostPerMS:            envFloat("COST_PER_MS", 0.01),
		OTELServiceName:      env("OTEL_SERVICE_NAME", "mevrelayv3"),
		OTELExporterEndpoint: env("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317"),
		OTELExporterInsecure: envBool("OTEL_EXPORTER_OTLP_INSECURE", true),
		OTELSampleRatio:      envFloat("OTEL_SAMPLE_RATIO", 0.25),
		APIAuthToken:         env("API_AUTH_TOKEN", ""),
	}
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return fallback
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
