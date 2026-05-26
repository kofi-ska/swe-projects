package config

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"txpool-builder/v1/internal/model"
)

func Load(args []string) (model.Config, error) {
	fs := flag.NewFlagSet("txpool-builder", flag.ContinueOnError)
	fs.SetOutput(ioDiscard{})

	var cfg model.Config
	var (
		configFile          = fs.String("config", "", "")
		rpcURL              = fs.String("rpc-url", "", "")
		output              = fs.String("output", "", "")
		traceOutput         = fs.String("trace-output", "", "")
		snapshotOutput      = fs.String("snapshot-output", "", "")
		replaySnapshot      = fs.String("replay-snapshot", "", "")
		compareCandidate    = fs.String("compare-candidate", "", "")
		timeout             = fs.Duration("timeout", 0, "")
		maxTransactions     = fs.Int("max-transactions", 0, "")
		maxGas              = fs.Uint64("max-gas", 0, "")
		maxSnapshotTxs      = fs.Int("max-snapshot-txs", 0, "")
		maxRawSnapshotBytes = fs.Int64("max-raw-snapshot-bytes", 0, "")
		maxArtifactBytes    = fs.Int64("max-artifact-bytes", 0, "")
		maxTraceBytes       = fs.Int64("max-trace-bytes", 0, "")
		policyVersion       = fs.String("policy-version", "", "")
		chainID             = fs.String("chain-id", "", "")
		strict              = fs.Bool("strict", true, "")
		rejectOnPartial     = fs.Bool("reject-on-partial-decode", true, "")
		allowHeadDrift      = fs.Bool("allow-head-drift", false, "")
		includeQueued       = fs.Bool("include-queued", false, "")
		noWrite             = fs.Bool("no-write", false, "")
		printConfig         = fs.Bool("print-config", false, "")
		dryRunConfig        = fs.Bool("dry-run-config", false, "")
		version             = fs.Bool("version", false, "")
	)

	if err := fs.Parse(args); err != nil {
		return model.Config{}, err
	}

	visited := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		visited[f.Name] = true
	})

	configPath := firstNonEmpty(*configFile, os.Getenv("TXPOOL_BUILDER_CONFIG"))
	fileCfg, fileDigest, err := loadConfigFile(configPath, *strict, visited["strict"])
	if err != nil {
		return model.Config{}, &model.StartupError{Code: model.ReasonConfigError, Stage: "config", Detail: err.Error(), Err: err}
	}

	cfg = fileCfg
	cfg.ConfigPath = configPath
	cfg = overlayEnv(cfg)
	cfg = overlayFlags(cfg, visited, map[string]any{
		"rpc-url":                  *rpcURL,
		"output":                   *output,
		"trace-output":             *traceOutput,
		"snapshot-output":          *snapshotOutput,
		"replay-snapshot":          *replaySnapshot,
		"compare-candidate":        *compareCandidate,
		"timeout":                  *timeout,
		"max-transactions":         *maxTransactions,
		"max-gas":                  *maxGas,
		"max-snapshot-txs":         *maxSnapshotTxs,
		"max-raw-snapshot-bytes":   *maxRawSnapshotBytes,
		"max-artifact-bytes":       *maxArtifactBytes,
		"max-trace-bytes":          *maxTraceBytes,
		"policy-version":           *policyVersion,
		"chain-id":                 *chainID,
		"strict":                   *strict,
		"reject-on-partial-decode": *rejectOnPartial,
		"allow-head-drift":         *allowHeadDrift,
		"include-queued":           *includeQueued,
		"no-write":                 *noWrite,
		"dry-run-config":           *dryRunConfig,
		"print-config":             *printConfig,
		"version":                  *version,
	})
	cfg.ConfigPath = configPath
	cfg.ReplaySnapshotPath = firstNonEmpty(cfg.ReplaySnapshotPath, os.Getenv("TXPOOL_BUILDER_REPLAY_SNAPSHOT"))
	cfg.CompareCandidatePath = firstNonEmpty(*compareCandidate, os.Getenv("TXPOOL_BUILDER_COMPARE_CANDIDATE"))

	if cfg.SnapshotOutputPath == "" && cfg.OutputPath != "" {
		cfg.SnapshotOutputPath = cfg.OutputPath + ".snapshot.json"
	}

	if err := Validate(cfg); err != nil {
		return model.Config{}, &model.StartupError{Code: model.ReasonConfigError, Stage: "config", Detail: err.Error(), Err: err}
	}
	_ = fileDigest
	return cfg, nil
}

type fileConfig struct {
	RPCURL               *string `json:"rpc_url"`
	OutputPath           *string `json:"output_path"`
	TraceOutputPath      *string `json:"trace_output_path"`
	SnapshotOutputPath   *string `json:"snapshot_output_path"`
	ReplaySnapshotPath   *string `json:"replay_snapshot_path"`
	CompareCandidatePath *string `json:"compare_candidate_path"`
	Timeout              *string `json:"timeout"`
	MaxTransactions      *int    `json:"max_transactions"`
	MaxGas               *uint64 `json:"max_gas"`
	MaxSnapshotTxs       *int    `json:"max_snapshot_txs"`
	MaxRawSnapshotBytes  *int64  `json:"max_raw_snapshot_bytes"`
	MaxArtifactBytes     *int64  `json:"max_artifact_bytes"`
	MaxTraceBytes        *int64  `json:"max_trace_bytes"`
	PolicyVersion        *string `json:"policy_version"`
	ChainID              *string `json:"chain_id"`
	Strict               *bool   `json:"strict"`
	RejectOnPartial      *bool   `json:"reject_on_partial_decode"`
	AllowHeadDrift       *bool   `json:"allow_head_drift"`
	IncludeQueued        *bool   `json:"include_queued"`
	NoWrite              *bool   `json:"no_write"`
	DryRunConfig         *bool   `json:"dry_run_config"`
	PrintConfig          *bool   `json:"print_config"`
	Version              *bool   `json:"version"`
}

func loadConfigFile(path string, strictFlag bool, strictVisited bool) (model.Config, string, error) {
	if strings.TrimSpace(path) == "" {
		return model.Config{}, "", nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return model.Config{}, "", err
	}
	var strictProbe struct {
		Strict *bool `json:"strict"`
	}
	_ = json.Unmarshal(b, &strictProbe)
	effectiveStrict := strictFlag
	if !strictVisited && strictProbe.Strict != nil {
		effectiveStrict = *strictProbe.Strict
	}
	if effectiveStrict {
		var unknown map[string]json.RawMessage
		if err := json.Unmarshal(b, &unknown); err != nil {
			return model.Config{}, "", err
		}
		allowed := map[string]struct{}{
			"rpc_url": {}, "output_path": {}, "trace_output_path": {}, "snapshot_output_path": {},
			"replay_snapshot_path": {}, "compare_candidate_path": {}, "timeout": {}, "max_transactions": {},
			"max_gas": {}, "max_snapshot_txs": {}, "max_raw_snapshot_bytes": {}, "max_artifact_bytes": {},
			"max_trace_bytes": {}, "policy_version": {}, "chain_id": {}, "strict": {}, "reject_on_partial_decode": {},
			"allow_head_drift": {}, "include_queued": {}, "no_write": {}, "dry_run_config": {}, "print_config": {}, "version": {},
		}
		for k := range unknown {
			if _, ok := allowed[k]; !ok {
				return model.Config{}, "", fmt.Errorf("unknown config key %q", k)
			}
		}
	}
	var fc fileConfig
	if err := json.Unmarshal(b, &fc); err != nil {
		return model.Config{}, "", err
	}
	cfg := model.Config{}
	applyFileConfig(&cfg, fc)
	d := Digest(cfg)
	return cfg, d, nil
}

func applyFileConfig(cfg *model.Config, fc fileConfig) {
	if fc.RPCURL != nil {
		cfg.RPCURL = *fc.RPCURL
	}
	if fc.OutputPath != nil {
		cfg.OutputPath = *fc.OutputPath
	}
	if fc.TraceOutputPath != nil {
		cfg.TraceOutputPath = *fc.TraceOutputPath
	}
	if fc.SnapshotOutputPath != nil {
		cfg.SnapshotOutputPath = *fc.SnapshotOutputPath
	}
	if fc.ReplaySnapshotPath != nil {
		cfg.ReplaySnapshotPath = *fc.ReplaySnapshotPath
	}
	if fc.CompareCandidatePath != nil {
		cfg.CompareCandidatePath = *fc.CompareCandidatePath
	}
	if fc.Timeout != nil {
		cfg.Timeout = parseDuration(*fc.Timeout)
	}
	if fc.MaxTransactions != nil {
		cfg.MaxTransactions = *fc.MaxTransactions
	}
	if fc.MaxGas != nil {
		cfg.MaxGas = *fc.MaxGas
	}
	if fc.MaxSnapshotTxs != nil {
		cfg.MaxSnapshotTxs = *fc.MaxSnapshotTxs
	}
	if fc.MaxRawSnapshotBytes != nil {
		cfg.MaxRawSnapshotBytes = *fc.MaxRawSnapshotBytes
	}
	if fc.MaxArtifactBytes != nil {
		cfg.MaxArtifactBytes = *fc.MaxArtifactBytes
	}
	if fc.MaxTraceBytes != nil {
		cfg.MaxTraceBytes = *fc.MaxTraceBytes
	}
	if fc.PolicyVersion != nil {
		cfg.PolicyVersion = *fc.PolicyVersion
	}
	if fc.ChainID != nil {
		cfg.ChainID = parseBigInt(*fc.ChainID)
	}
	if fc.Strict != nil {
		cfg.Strict = *fc.Strict
	}
	if fc.RejectOnPartial != nil {
		cfg.RejectOnPartialDecode = *fc.RejectOnPartial
	}
	if fc.AllowHeadDrift != nil {
		cfg.AllowHeadDrift = *fc.AllowHeadDrift
	}
	if fc.IncludeQueued != nil {
		cfg.IncludeQueued = *fc.IncludeQueued
	}
	if fc.NoWrite != nil {
		cfg.NoWrite = *fc.NoWrite
	}
	if fc.DryRunConfig != nil {
		cfg.DryRunConfig = *fc.DryRunConfig
	}
	if fc.PrintConfig != nil {
		cfg.PrintConfig = *fc.PrintConfig
	}
	if fc.Version != nil {
		cfg.Version = *fc.Version
	}
}

func overlayEnv(cfg model.Config) model.Config {
	if v := firstNonEmpty(os.Getenv("TXPOOL_BUILDER_RPC_URL"), os.Getenv("GETH_RPC_URL")); v != "" {
		cfg.RPCURL = v
	}
	if v := os.Getenv("TXPOOL_BUILDER_OUTPUT"); v != "" {
		cfg.OutputPath = v
	}
	if v := os.Getenv("TXPOOL_BUILDER_TRACE_OUTPUT"); v != "" {
		cfg.TraceOutputPath = v
	}
	if v := os.Getenv("TXPOOL_BUILDER_SNAPSHOT_OUTPUT"); v != "" {
		cfg.SnapshotOutputPath = v
	}
	if v := os.Getenv("TXPOOL_BUILDER_REPLAY_SNAPSHOT"); v != "" {
		cfg.ReplaySnapshotPath = v
	}
	if v := os.Getenv("TXPOOL_BUILDER_COMPARE_CANDIDATE"); v != "" {
		cfg.CompareCandidatePath = v
	}
	if v := os.Getenv("TXPOOL_BUILDER_TIMEOUT"); v != "" {
		cfg.Timeout = parseDuration(v)
	}
	if v := os.Getenv("TXPOOL_BUILDER_MAX_TRANSACTIONS"); v != "" {
		cfg.MaxTransactions = parseInt(v)
	}
	if v := os.Getenv("TXPOOL_BUILDER_MAX_GAS"); v != "" {
		cfg.MaxGas = parseUint64(v)
	}
	if v := os.Getenv("TXPOOL_BUILDER_MAX_SNAPSHOT_TXS"); v != "" {
		cfg.MaxSnapshotTxs = parseInt(v)
	}
	if v := os.Getenv("TXPOOL_BUILDER_MAX_RAW_SNAPSHOT_BYTES"); v != "" {
		cfg.MaxRawSnapshotBytes = parseInt64(v)
	}
	if v := os.Getenv("TXPOOL_BUILDER_MAX_ARTIFACT_BYTES"); v != "" {
		cfg.MaxArtifactBytes = parseInt64(v)
	}
	if v := os.Getenv("TXPOOL_BUILDER_MAX_TRACE_BYTES"); v != "" {
		cfg.MaxTraceBytes = parseInt64(v)
	}
	if v := os.Getenv("TXPOOL_BUILDER_POLICY_VERSION"); v != "" {
		cfg.PolicyVersion = v
	}
	if v := os.Getenv("TXPOOL_BUILDER_CHAIN_ID"); v != "" {
		cfg.ChainID = parseBigInt(v)
	}
	return cfg
}

func overlayFlags(cfg model.Config, visited map[string]bool, vals map[string]any) model.Config {
	if visited["rpc-url"] {
		cfg.RPCURL = vals["rpc-url"].(string)
	}
	if visited["output"] {
		cfg.OutputPath = vals["output"].(string)
	}
	if visited["trace-output"] {
		cfg.TraceOutputPath = vals["trace-output"].(string)
	}
	if visited["snapshot-output"] {
		cfg.SnapshotOutputPath = vals["snapshot-output"].(string)
	}
	if visited["replay-snapshot"] {
		cfg.ReplaySnapshotPath = vals["replay-snapshot"].(string)
	}
	if visited["compare-candidate"] {
		cfg.CompareCandidatePath = vals["compare-candidate"].(string)
	}
	if visited["timeout"] {
		cfg.Timeout = vals["timeout"].(time.Duration)
	}
	if visited["max-transactions"] {
		cfg.MaxTransactions = vals["max-transactions"].(int)
	}
	if visited["max-gas"] {
		cfg.MaxGas = vals["max-gas"].(uint64)
	}
	if visited["max-snapshot-txs"] {
		cfg.MaxSnapshotTxs = vals["max-snapshot-txs"].(int)
	}
	if visited["max-raw-snapshot-bytes"] {
		cfg.MaxRawSnapshotBytes = vals["max-raw-snapshot-bytes"].(int64)
	}
	if visited["max-artifact-bytes"] {
		cfg.MaxArtifactBytes = vals["max-artifact-bytes"].(int64)
	}
	if visited["max-trace-bytes"] {
		cfg.MaxTraceBytes = vals["max-trace-bytes"].(int64)
	}
	if visited["policy-version"] {
		cfg.PolicyVersion = vals["policy-version"].(string)
	}
	if visited["chain-id"] {
		cfg.ChainID = parseBigInt(vals["chain-id"].(string))
	}
	if visited["strict"] {
		cfg.Strict = vals["strict"].(bool)
	}
	if visited["reject-on-partial-decode"] {
		cfg.RejectOnPartialDecode = vals["reject-on-partial-decode"].(bool)
	}
	if visited["allow-head-drift"] {
		cfg.AllowHeadDrift = vals["allow-head-drift"].(bool)
	}
	if visited["include-queued"] {
		cfg.IncludeQueued = vals["include-queued"].(bool)
	}
	if visited["no-write"] {
		cfg.NoWrite = vals["no-write"].(bool)
	}
	if visited["dry-run-config"] {
		cfg.DryRunConfig = vals["dry-run-config"].(bool)
	}
	if visited["print-config"] {
		cfg.PrintConfig = vals["print-config"].(bool)
	}
	if visited["version"] {
		cfg.Version = vals["version"].(bool)
	}
	return cfg
}

func Validate(cfg model.Config) error {
	switch {
	case cfg.RPCURL == "" && cfg.ReplaySnapshotPath == "":
		return errors.New("missing RPC URL")
	case cfg.OutputPath == "" && !cfg.NoWrite:
		return errors.New("missing output path")
	case cfg.TraceOutputPath == "" && !cfg.NoWrite:
		return errors.New("missing trace output path")
	case cfg.SnapshotOutputPath == "" && !cfg.NoWrite:
		return errors.New("missing snapshot output path")
	case cfg.Timeout <= 0:
		return errors.New("missing or invalid timeout")
	case cfg.MaxTransactions <= 0:
		return errors.New("missing or invalid max-transactions")
	case cfg.MaxGas == 0:
		return errors.New("missing or invalid max-gas")
	case cfg.MaxSnapshotTxs <= 0:
		return errors.New("missing or invalid max-snapshot-txs")
	case cfg.MaxRawSnapshotBytes <= 0:
		return errors.New("missing or invalid max-raw-snapshot-bytes")
	case cfg.MaxArtifactBytes <= 0:
		return errors.New("missing or invalid max-artifact-bytes")
	case cfg.MaxTraceBytes <= 0:
		return errors.New("missing or invalid max-trace-bytes")
	case cfg.PolicyVersion == "":
		return errors.New("missing policy version")
	case cfg.ChainID == nil && cfg.ReplaySnapshotPath == "":
		return errors.New("missing or invalid chain id")
	}

	if strings.TrimSpace(cfg.RPCURL) == "" {
		return errors.New("missing RPC URL")
	}

	if err := validateParentDir(cfg.OutputPath); err != nil {
		return err
	}
	if err := validateParentDir(cfg.TraceOutputPath); err != nil {
		return err
	}
	if err := validateParentDir(cfg.SnapshotOutputPath); err != nil {
		return err
	}
	if err := validateParentDir(cfg.ReplaySnapshotPath); err != nil {
		return err
	}

	return nil
}

func validateParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func firstDuration(flagValue time.Duration, envValue string) time.Duration {
	if flagValue > 0 {
		return flagValue
	}
	if envValue == "" {
		return 0
	}
	d, _ := time.ParseDuration(envValue)
	return d
}

func parseDuration(s string) time.Duration {
	d, _ := time.ParseDuration(strings.TrimSpace(s))
	return d
}

func parseInt(s string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}

func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return v
}

func parseUint64(s string) uint64 {
	v, _ := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	return v
}

func firstInt(flagValue int, envValue string) int {
	if flagValue > 0 {
		return flagValue
	}
	if envValue == "" {
		return 0
	}
	v, _ := strconv.Atoi(envValue)
	return v
}

func firstInt64(flagValue int64, envValue string) int64 {
	if flagValue > 0 {
		return flagValue
	}
	if envValue == "" {
		return 0
	}
	v, _ := strconv.ParseInt(envValue, 10, 64)
	return v
}

func firstUint64(flagValue uint64, envValue string) uint64 {
	if flagValue > 0 {
		return flagValue
	}
	if envValue == "" {
		return 0
	}
	v, _ := strconv.ParseUint(envValue, 10, 64)
	return v
}

func parseBigInt(s string) *big.Int {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		n, ok := new(big.Int).SetString(s[2:], 16)
		if ok {
			return n
		}
		return nil
	}
	n, ok := new(big.Int).SetString(s, 10)
	if ok {
		return n
	}
	return nil
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func Digest(cfg model.Config) string {
	type digestConfig struct {
		ConfigPath           string `json:"config_path"`
		RPCURL               string `json:"rpc_url"`
		OutputPath           string `json:"output_path"`
		TraceOutputPath      string `json:"trace_output_path"`
		SnapshotOutputPath   string `json:"snapshot_output_path"`
		ReplaySnapshotPath   string `json:"replay_snapshot_path"`
		CompareCandidatePath string `json:"compare_candidate_path"`
		Timeout              string `json:"timeout"`
		MaxTransactions      int    `json:"max_transactions"`
		MaxGas               uint64 `json:"max_gas"`
		MaxSnapshotTxs       int    `json:"max_snapshot_txs"`
		MaxRawSnapshotBytes  int64  `json:"max_raw_snapshot_bytes"`
		MaxArtifactBytes     int64  `json:"max_artifact_bytes"`
		MaxTraceBytes        int64  `json:"max_trace_bytes"`
		PolicyVersion        string `json:"policy_version"`
		ChainID              string `json:"chain_id"`
		Strict               bool   `json:"strict"`
		RejectOnPartial      bool   `json:"reject_on_partial_decode"`
		AllowHeadDrift       bool   `json:"allow_head_drift"`
		IncludeQueued        bool   `json:"include_queued"`
		NoWrite              bool   `json:"no_write"`
		DryRunConfig         bool   `json:"dry_run_config"`
	}
	j, _ := json.Marshal(digestConfig{
		ConfigPath:           cfg.ConfigPath,
		RPCURL:               cfg.RPCURL,
		OutputPath:           cfg.OutputPath,
		TraceOutputPath:      cfg.TraceOutputPath,
		SnapshotOutputPath:   cfg.SnapshotOutputPath,
		ReplaySnapshotPath:   cfg.ReplaySnapshotPath,
		CompareCandidatePath: cfg.CompareCandidatePath,
		Timeout:              cfg.Timeout.String(),
		MaxTransactions:      cfg.MaxTransactions,
		MaxGas:               cfg.MaxGas,
		MaxSnapshotTxs:       cfg.MaxSnapshotTxs,
		MaxRawSnapshotBytes:  cfg.MaxRawSnapshotBytes,
		MaxArtifactBytes:     cfg.MaxArtifactBytes,
		MaxTraceBytes:        cfg.MaxTraceBytes,
		PolicyVersion:        cfg.PolicyVersion,
		ChainID:              chainIDString(cfg.ChainID),
		Strict:               cfg.Strict,
		RejectOnPartial:      cfg.RejectOnPartialDecode,
		AllowHeadDrift:       cfg.AllowHeadDrift,
		IncludeQueued:        cfg.IncludeQueued,
		NoWrite:              cfg.NoWrite,
		DryRunConfig:         cfg.DryRunConfig,
	})
	sum := sha256.Sum256(j)
	return fmt.Sprintf("%x", sum[:])
}

func chainIDString(v *big.Int) string {
	if v == nil {
		return ""
	}
	return v.String()
}
