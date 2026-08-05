package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPPort, PprofAddress, OTLPEndpoint                              string
	QueueCapacity, WorkerCount, MaxBatchSize                          int
	MaxBatchWait, RequestTimeout, StreamWriteTimeout, ShutdownTimeout time.Duration
	BaseInferenceDelay, PerTokenDelay                                 time.Duration
	FailureRate, OTELSampleRate                                       float64
	ReadinessSaturation                                               float64
}

func Load() (Config, error) {
	c := Config{
		HTTPPort: env("HTTP_PORT", "8080"), PprofAddress: env("PPROF_ADDRESS", "localhost:6060"),
		OTLPEndpoint: env("OTEL_EXPORTER_OTLP_ENDPOINT", "jaeger:4317"), QueueCapacity: envInt("QUEUE_CAPACITY", 1000),
		WorkerCount: envInt("WORKER_COUNT", 8), MaxBatchSize: envInt("MAX_BATCH_SIZE", 8),
		MaxBatchWait: envDuration("MAX_BATCH_WAIT", 20*time.Millisecond), RequestTimeout: envDuration("REQUEST_TIMEOUT", 10*time.Second), StreamWriteTimeout: envDuration("STREAM_WRITE_TIMEOUT", time.Second),
		ShutdownTimeout: envDuration("SHUTDOWN_TIMEOUT", 15*time.Second), BaseInferenceDelay: envDuration("BASE_INFERENCE_DELAY", 50*time.Millisecond),
		PerTokenDelay: envDuration("PER_TOKEN_DELAY", 10*time.Millisecond), FailureRate: envFloat("FAILURE_RATE", .01),
		OTELSampleRate: envFloat("OTEL_SAMPLE_RATE", 1), ReadinessSaturation: envFloat("READINESS_SATURATION", .8),
	}
	if c.QueueCapacity < 1 || c.WorkerCount < 1 || c.MaxBatchSize < 1 {
		return c, fmt.Errorf("queue capacity, worker count, and batch size must be positive")
	}
	if c.MaxBatchWait <= 0 || c.RequestTimeout <= 0 || c.StreamWriteTimeout <= 0 || c.ShutdownTimeout <= 0 || c.BaseInferenceDelay < 0 || c.PerTokenDelay < 0 {
		return c, fmt.Errorf("durations are invalid")
	}
	if c.FailureRate < 0 || c.FailureRate > 1 || c.OTELSampleRate < 0 || c.OTELSampleRate > 1 || c.ReadinessSaturation <= 0 || c.ReadinessSaturation > 1 {
		return c, fmt.Errorf("rates must be between 0 and 1")
	}
	return c, nil
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func envInt(k string, d int) int {
	v, e := strconv.Atoi(env(k, strconv.Itoa(d)))
	if e != nil {
		return -1
	}
	return v
}
func envFloat(k string, d float64) float64 {
	v, e := strconv.ParseFloat(env(k, fmt.Sprint(d)), 64)
	if e != nil {
		return -1
	}
	return v
}
func envDuration(k string, d time.Duration) time.Duration {
	v, e := time.ParseDuration(env(k, d.String()))
	if e != nil {
		return -1
	}
	return v
}
