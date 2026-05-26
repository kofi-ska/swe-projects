package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math/big"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"txpool-builder/v2/internal/model"
)

func Load(args []string) (model.Config, error) {
	fs := flag.NewFlagSet("txpool-builder-v2", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})

	var cfg model.Config
	listenAddr := fs.String("listen", ":8080", "")
	rpcURL := fs.String("rpc-url", "", "")
	outputDir := fs.String("output-dir", "./out", "")
	policyVersion := fs.String("policy-version", "v2", "")
	chainID := fs.String("chain-id", "", "")
	refreshInterval := fs.Duration("refresh-interval", 5*time.Second, "")
	requestTimeout := fs.Duration("request-timeout", 30*time.Second, "")
	admissionTimeout := fs.Duration("admission-timeout", 250*time.Millisecond, "")
	queueSize := fs.Int("queue-size", 1024, "")
	workerCount := fs.Int("workers", max(2, runtime.NumCPU()/2), "")
	maxRetainedJobs := fs.Int("max-retained-jobs", 1024, "")
	maxRetainedSnap := fs.Int("max-retained-snapshots", 4, "")
	maxArtifactBytes := fs.Int64("max-artifact-bytes", 8<<20, "")
	maxTraceBytes := fs.Int64("max-trace-bytes", 2<<20, "")
	maxSnapshotBytes := fs.Int64("max-snapshot-bytes", 32<<20, "")
	maxSnapshotAge := fs.Duration("max-snapshot-age", 15*time.Second, "")
	maxRPCPerRequest := fs.Int("max-rpc-per-request", 1, "")
	maxRetryAttempts := fs.Int("max-retry-attempts", 2, "")
	maxGas := fs.Uint64("max-gas", 0, "")
	maxTransactions := fs.Int("max-transactions", 64, "")
	includeQueued := fs.Bool("include-queued", false, "")
	allowHeadDrift := fs.Bool("allow-head-drift", false, "")
	strict := fs.Bool("strict", true, "")
	noWrite := fs.Bool("no-write", false, "")
	printConfig := fs.Bool("print-config", false, "")
	dryRunConfig := fs.Bool("dry-run-config", false, "")
	version := fs.Bool("version", false, "")

	if err := fs.Parse(args); err != nil {
		return model.Config{}, err
	}

	cfg.ListenAddr = firstNonEmpty(*listenAddr, os.Getenv("TXPOOL_BUILDER_LISTEN"))
	cfg.RPCURL = firstNonEmpty(*rpcURL, os.Getenv("TXPOOL_BUILDER_RPC_URL"), os.Getenv("GETH_RPC_URL"))
	cfg.OutputDir = firstNonEmpty(*outputDir, os.Getenv("TXPOOL_BUILDER_OUTPUT_DIR"), "./out")
	cfg.PolicyVersion = firstNonEmpty(*policyVersion, os.Getenv("TXPOOL_BUILDER_POLICY_VERSION"), "v2")
	cfg.RefreshInterval = durationEnv("TXPOOL_BUILDER_REFRESH_INTERVAL", *refreshInterval)
	cfg.RequestTimeout = durationEnv("TXPOOL_BUILDER_REQUEST_TIMEOUT", *requestTimeout)
	cfg.AdmissionTimeout = durationEnv("TXPOOL_BUILDER_ADMISSION_TIMEOUT", *admissionTimeout)
	cfg.QueueSize = intEnv("TXPOOL_BUILDER_QUEUE_SIZE", *queueSize)
	cfg.WorkerCount = intEnv("TXPOOL_BUILDER_WORKERS", *workerCount)
	cfg.MaxRetainedJobs = intEnv("TXPOOL_BUILDER_MAX_RETAINED_JOBS", *maxRetainedJobs)
	cfg.MaxRetainedSnap = intEnv("TXPOOL_BUILDER_MAX_RETAINED_SNAPSHOTS", *maxRetainedSnap)
	cfg.MaxArtifactBytes = int64Env("TXPOOL_BUILDER_MAX_ARTIFACT_BYTES", *maxArtifactBytes)
	cfg.MaxTraceBytes = int64Env("TXPOOL_BUILDER_MAX_TRACE_BYTES", *maxTraceBytes)
	cfg.MaxSnapshotBytes = int64Env("TXPOOL_BUILDER_MAX_SNAPSHOT_BYTES", *maxSnapshotBytes)
	cfg.MaxSnapshotAge = durationEnv("TXPOOL_BUILDER_MAX_SNAPSHOT_AGE", *maxSnapshotAge)
	cfg.MaxRPCPerRequest = intEnv("TXPOOL_BUILDER_MAX_RPC_PER_REQUEST", *maxRPCPerRequest)
	cfg.MaxRetryAttempts = intEnv("TXPOOL_BUILDER_MAX_RETRY_ATTEMPTS", *maxRetryAttempts)
	cfg.MaxGas = uint64Env("TXPOOL_BUILDER_MAX_GAS", *maxGas)
	cfg.MaxTransactions = intEnv("TXPOOL_BUILDER_MAX_TRANSACTIONS", *maxTransactions)
	cfg.IncludeQueued = boolEnv("TXPOOL_BUILDER_INCLUDE_QUEUED", *includeQueued)
	cfg.AllowHeadDrift = boolEnv("TXPOOL_BUILDER_ALLOW_HEAD_DRIFT", *allowHeadDrift)
	cfg.Strict = boolEnv("TXPOOL_BUILDER_STRICT", *strict)
	cfg.NoWrite = boolEnv("TXPOOL_BUILDER_NO_WRITE", *noWrite)
	cfg.PrintConfig = boolEnv("TXPOOL_BUILDER_PRINT_CONFIG", *printConfig)
	cfg.DryRunConfig = boolEnv("TXPOOL_BUILDER_DRY_RUN_CONFIG", *dryRunConfig)
	cfg.Version = boolEnv("TXPOOL_BUILDER_VERSION", *version)

	if chainIDEnv := firstNonEmpty(*chainID, os.Getenv("TXPOOL_BUILDER_CHAIN_ID")); chainIDEnv != "" {
		cfg.ChainID = parseBigInt(chainIDEnv)
	}

	if err := Validate(cfg); err != nil {
		return model.Config{}, &model.StartupError{Code: model.ReasonConfigError, Stage: "config", Detail: err.Error(), Err: err}
	}
	return cfg, nil
}

func Validate(cfg model.Config) error {
	switch {
	case cfg.ListenAddr == "":
		return fmt.Errorf("listen address is required")
	case cfg.RPCURL == "":
		return fmt.Errorf("rpc url is required")
	case cfg.OutputDir == "":
		return fmt.Errorf("output dir is required")
	case cfg.PolicyVersion == "":
		return fmt.Errorf("policy version is required")
	case cfg.ChainID == nil || cfg.ChainID.Sign() <= 0:
		return fmt.Errorf("chain id is required")
	case cfg.RefreshInterval <= 0:
		return fmt.Errorf("refresh interval must be positive")
	case cfg.RequestTimeout <= 0:
		return fmt.Errorf("request timeout must be positive")
	case cfg.AdmissionTimeout <= 0:
		return fmt.Errorf("admission timeout must be positive")
	case cfg.QueueSize <= 0:
		return fmt.Errorf("queue size must be positive")
	case cfg.WorkerCount <= 0:
		return fmt.Errorf("workers must be positive")
	case cfg.MaxRetainedJobs <= 0:
		return fmt.Errorf("max retained jobs must be positive")
	case cfg.MaxRetainedSnap <= 0:
		return fmt.Errorf("max retained snapshots must be positive")
	case cfg.MaxArtifactBytes <= 0:
		return fmt.Errorf("max artifact bytes must be positive")
	case cfg.MaxTraceBytes <= 0:
		return fmt.Errorf("max trace bytes must be positive")
	case cfg.MaxSnapshotBytes <= 0:
		return fmt.Errorf("max snapshot bytes must be positive")
	case cfg.MaxSnapshotAge <= 0:
		return fmt.Errorf("max snapshot age must be positive")
	case cfg.MaxRPCPerRequest <= 0:
		return fmt.Errorf("max rpc per request must be positive")
	case cfg.MaxRetryAttempts < 0:
		return fmt.Errorf("max retry attempts cannot be negative")
	case cfg.MaxTransactions <= 0:
		return fmt.Errorf("max transactions must be positive")
	case cfg.MaxGas > 0 && cfg.MaxTransactions == 0:
		return fmt.Errorf("max transactions must be positive when max gas is set")
	}
	return nil
}

func Digest(cfg model.Config) string {
	b, _ := json.Marshal(cfg)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func parseBigInt(s string) *big.Int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		if n, ok := new(big.Int).SetString(s[2:], 16); ok {
			return n
		}
		return nil
	}
	if n, ok := new(big.Int).SetString(s, 10); ok {
		return n
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func int64Env(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func uint64Env(key string, fallback uint64) uint64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

func boolEnv(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return fallback
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
