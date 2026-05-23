package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds the bounded runtime settings for v2.
type Config struct {
	HTTPAddr             string
	DataDir              string
	RegionID             string
	BackendKind          string
	BackendURL           string
	StateKind            string
	ValkeyURL            string
	BrokerKind           string
	BrokerTopic          string
	BrokerBuffer         int
	NATSURL              string
	QueueDepth           int
	WorkerCount          int
	MaxRetries           int
	RetryBackoff         time.Duration
	AuditFlushEvery      int
	WALMaxEntries        int
	MaxPayloadBytes      int64
	MaxInFlightPerClient int
	RequestTimeout       time.Duration
	HistoryLimit         int
	StateRetention       time.Duration
	MinNetValue          float64
	MinDeadlineSlack     time.Duration
	MaxQueueAge          time.Duration
	ValuePerTx           float64
	CostPerTx            float64
	CostPerMS            float64
}

// Load reads runtime settings with conservative defaults.
func Load() Config {
	return Config{
		HTTPAddr:             env("HTTP_ADDR", ":8090"),
		DataDir:              env("DATA_DIR", "data"),
		RegionID:             env("REGION_ID", "local"),
		BackendKind:          env("BACKEND_KIND", "anvil"),
		BackendURL:           env("BACKEND_URL", "http://127.0.0.1:8545"),
		StateKind:            env("STATE_KIND", "valkey"),
		ValkeyURL:            env("VALKEY_URL", "redis://127.0.0.1:6379"),
		BrokerKind:           env("BROKER_KIND", "memory"),
		BrokerTopic:          env("BROKER_TOPIC", "mev.v2.events"),
		BrokerBuffer:         envInt("BROKER_BUFFER", 256),
		NATSURL:              env("NATS_URL", "nats://127.0.0.1:4222"),
		QueueDepth:           envInt("QUEUE_DEPTH", 1024),
		WorkerCount:          envInt("WORKER_COUNT", 4),
		MaxRetries:           envInt("MAX_RETRIES", 3),
		RetryBackoff:         envDuration("RETRY_BACKOFF", 500*time.Millisecond),
		AuditFlushEvery:      envInt("AUDIT_FLUSH_EVERY", 128),
		WALMaxEntries:        envInt("WAL_MAX_ENTRIES", 2048),
		MaxPayloadBytes:      int64(envInt("MAX_PAYLOAD_BYTES", 256*1024)),
		MaxInFlightPerClient: envInt("MAX_INFLIGHT_PER_CLIENT", 20),
		RequestTimeout:       envDuration("REQUEST_TIMEOUT", 2*time.Second),
		HistoryLimit:         envInt("HISTORY_LIMIT", 256),
		StateRetention:       envDuration("STATE_RETENTION", 24*time.Hour),
		MinNetValue:          envFloat("MIN_NET_VALUE", 0),
		MinDeadlineSlack:     envDuration("MIN_DEADLINE_SLACK", 500*time.Millisecond),
		MaxQueueAge:          envDuration("MAX_QUEUE_AGE", 3*time.Second),
		ValuePerTx:           envFloat("VALUE_PER_TX", 1),
		CostPerTx:            envFloat("COST_PER_TX", 0.25),
		CostPerMS:            envFloat("COST_PER_MS", 0.01),
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
		if n, err := strconv.Atoi(v); err == nil {
			if n < 0 {
				return fallback
			}
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
