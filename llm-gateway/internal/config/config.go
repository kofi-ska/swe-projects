package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPPort            string
	PprofAddress        string
	OTLPEndpoint        string
	QueueCapacity       int
	WorkerCount         int
	MaxBatchSize        int
	MaxBatchWait        time.Duration
	RequestTimeout      time.Duration
	StreamWriteTimeout  time.Duration
	ShutdownTimeout     time.Duration
	BaseInferenceDelay  time.Duration
	PerTokenDelay       time.Duration
	FailureRate         float64
	OTELSampleRate      float64
	ReadinessSaturation float64
}

func Load() (Config, error) {
	var config Config
	config.HTTPPort = value("HTTP_PORT", "8080")
	config.PprofAddress = value("PPROF_ADDRESS", "localhost:6060")
	config.OTLPEndpoint = value("OTEL_EXPORTER_OTLP_ENDPOINT", "jaeger:4317")

	var err error
	if config.QueueCapacity, err = integer("QUEUE_CAPACITY", 1000); err != nil {
		return Config{}, err
	}
	if config.WorkerCount, err = integer("WORKER_COUNT", 8); err != nil {
		return Config{}, err
	}
	if config.MaxBatchSize, err = integer("MAX_BATCH_SIZE", 8); err != nil {
		return Config{}, err
	}
	if config.MaxBatchWait, err = duration("MAX_BATCH_WAIT", 20*time.Millisecond); err != nil {
		return Config{}, err
	}
	if config.RequestTimeout, err = duration("REQUEST_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if config.StreamWriteTimeout, err = duration("STREAM_WRITE_TIMEOUT", time.Second); err != nil {
		return Config{}, err
	}
	if config.ShutdownTimeout, err = duration("SHUTDOWN_TIMEOUT", 15*time.Second); err != nil {
		return Config{}, err
	}
	if config.BaseInferenceDelay, err = duration("BASE_INFERENCE_DELAY", 50*time.Millisecond); err != nil {
		return Config{}, err
	}
	if config.PerTokenDelay, err = duration("PER_TOKEN_DELAY", 10*time.Millisecond); err != nil {
		return Config{}, err
	}
	if config.FailureRate, err = decimal("FAILURE_RATE", 0.01); err != nil {
		return Config{}, err
	}
	if config.OTELSampleRate, err = decimal("OTEL_SAMPLE_RATE", 1); err != nil {
		return Config{}, err
	}
	if config.ReadinessSaturation, err = decimal("READINESS_SATURATION", 0.8); err != nil {
		return Config{}, err
	}

	if config.QueueCapacity < 1 {
		return Config{}, fmt.Errorf("QUEUE_CAPACITY must be positive")
	}
	if config.WorkerCount < 1 {
		return Config{}, fmt.Errorf("WORKER_COUNT must be positive")
	}
	if config.MaxBatchSize < 1 {
		return Config{}, fmt.Errorf("MAX_BATCH_SIZE must be positive")
	}
	if config.MaxBatchWait <= 0 || config.RequestTimeout <= 0 || config.StreamWriteTimeout <= 0 || config.ShutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("wait and timeout durations must be positive")
	}
	if config.BaseInferenceDelay < 0 || config.PerTokenDelay < 0 {
		return Config{}, fmt.Errorf("inference delays must not be negative")
	}
	if err := unitInterval("FAILURE_RATE", config.FailureRate, true); err != nil {
		return Config{}, err
	}
	if err := unitInterval("OTEL_SAMPLE_RATE", config.OTELSampleRate, true); err != nil {
		return Config{}, err
	}
	if err := unitInterval("READINESS_SATURATION", config.ReadinessSaturation, false); err != nil {
		return Config{}, err
	}
	return config, nil
}

func value(key, fallback string) string {
	if raw, ok := os.LookupEnv(key); ok {
		return raw
	}
	return fallback
}

func integer(key string, fallback int) (int, error) {
	raw := value(key, strconv.Itoa(fallback))
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s=%q must be an integer: %w", key, raw, err)
	}
	return parsed, nil
}

func decimal(key string, fallback float64) (float64, error) {
	raw := value(key, strconv.FormatFloat(fallback, 'g', -1, 64))
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s=%q must be a number: %w", key, raw, err)
	}
	return parsed, nil
}

func duration(key string, fallback time.Duration) (time.Duration, error) {
	raw := value(key, fallback.String())
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s=%q must be a Go duration: %w", key, raw, err)
	}
	return parsed, nil
}

func unitInterval(key string, number float64, includeZero bool) error {
	if number > 1 || (includeZero && number < 0) || (!includeZero && number <= 0) {
		lowerBound := "greater than 0"
		if includeZero {
			lowerBound = "at least 0"
		}
		return fmt.Errorf("%s must be %s and at most 1", key, lowerBound)
	}
	return nil
}
