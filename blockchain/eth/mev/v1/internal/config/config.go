package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds runtime settings and bounded resource limits.
type Config struct {
	HTTPAddr             string
	RPCURL               string
	DataDir              string
	QueueDepth           int
	WorkerCount          int
	MaxRetries           int
	RetryBackoff         time.Duration
	MaxPayloadBytes      int64
	MaxInFlightPerClient int
	RequestTimeout       time.Duration
}

// Load reads configuration from the environment with safe defaults.
func Load() Config {
	return Config{
		HTTPAddr:             env("HTTP_ADDR", ":8080"),
		RPCURL:               env("RPC_URL", "http://127.0.0.1:8545"),
		DataDir:              env("DATA_DIR", "data"),
		QueueDepth:           envInt("QUEUE_DEPTH", 1024),
		WorkerCount:          envInt("WORKER_COUNT", 4),
		MaxRetries:           envInt("MAX_RETRIES", 3),
		RetryBackoff:         envDuration("RETRY_BACKOFF", 500*time.Millisecond),
		MaxPayloadBytes:      int64(envInt("MAX_PAYLOAD_BYTES", 256*1024)),
		MaxInFlightPerClient: envInt("MAX_INFLIGHT_PER_CLIENT", 20),
		RequestTimeout:       envDuration("REQUEST_TIMEOUT", 2*time.Second),
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
