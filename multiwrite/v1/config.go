package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	WriterDirect   = "direct"
	WriterBuffered = "buffered"

	ProfileFixed       = "fixed"
	ProfileTiny        = "tiny"
	ProfileWide        = "wide"
	ProfileHeavyTail   = "heavy-tail"
	ProfileHighEntropy = "high-entropy"
	ProfileLowEntropy  = "low-entropy"
	ProfileBurst       = "burst"

	DurabilityClosed    = "closed"
	DurabilitySynced    = "synced"
	DurabilityCommitted = "committed"

	OverwriteFail    = "fail"
	OverwriteReplace = "replace"
	OverwriteVersion = "version"
)

const (
	defaultRecordsLimit = int64(1_000_000)
	defaultMaxBytes     = int64(256 * 1024 * 1024)
	defaultDuration     = 30 * time.Second
	defaultRecordSize   = 32
	defaultBufferSize   = 16 * 1024
	defaultBatchSize    = 64 * 1024
	defaultSeed         = int64(1)
)

type RunConfig struct {
	RecordsLimit int64         `json:"records_limit"`
	MaxBytes     int64         `json:"max_bytes"`
	MaxDuration  time.Duration `json:"max_duration"`
	Writer       string        `json:"writer_strategy"`
	Profile      string        `json:"workload_profile"`
	Seed         int64         `json:"seed"`
	RecordSize   int           `json:"record_size"`
	BufferSize   int           `json:"buffer_size"`
	BatchSize    int           `json:"batch_size"`
	QueueDepth   int           `json:"queue_depth"`
	OutputPath   string        `json:"output_path"`
	ManifestPath string        `json:"manifest_path"`
	Durability   string        `json:"durability_mode"`
	Overwrite    string        `json:"overwrite_policy"`
}

func DefaultRunConfig() RunConfig {
	return RunConfig{
		RecordsLimit: defaultRecordsLimit,
		MaxBytes:     defaultMaxBytes,
		MaxDuration:  defaultDuration,
		Writer:       WriterBuffered,
		Profile:      ProfileFixed,
		Seed:         defaultSeed,
		RecordSize:   defaultRecordSize,
		BufferSize:   defaultBufferSize,
		BatchSize:    defaultBatchSize,
		QueueDepth:   0,
		OutputPath:   "benchmarks/artifacts/outputs/words.txt",
		ManifestPath: "benchmarks/artifacts/manifests/run.json",
		Durability:   DurabilityClosed,
		Overwrite:    OverwriteFail,
	}
}

func (c *RunConfig) ApplyDefaults() {
	if c.RecordsLimit == 0 {
		c.RecordsLimit = defaultRecordsLimit
	}
	if c.MaxBytes == 0 {
		c.MaxBytes = defaultMaxBytes
	}
	if c.MaxDuration == 0 {
		c.MaxDuration = defaultDuration
	}
	if c.Writer == "" {
		c.Writer = WriterBuffered
	}
	if c.Profile == "" {
		c.Profile = ProfileFixed
	}
	if c.RecordSize == 0 {
		c.RecordSize = defaultRecordSize
	}
	if c.BufferSize == 0 {
		c.BufferSize = defaultBufferSize
	}
	if c.BatchSize == 0 {
		c.BatchSize = defaultBatchSize
	}
	if c.OutputPath == "" {
		c.OutputPath = "benchmarks/artifacts/outputs/words.txt"
	}
	if c.ManifestPath == "" {
		c.ManifestPath = "benchmarks/artifacts/manifests/run.json"
	}
	if c.Durability == "" {
		c.Durability = DurabilityClosed
	}
	if c.Overwrite == "" {
		c.Overwrite = OverwriteFail
	}
}

func (c RunConfig) Validate() error {
	if c.RecordsLimit <= 0 {
		return fmt.Errorf("%w: records limit must be positive", ErrValidation)
	}
	if c.MaxBytes <= 0 {
		return fmt.Errorf("%w: max bytes must be positive", ErrValidation)
	}
	if c.MaxDuration <= 0 {
		return fmt.Errorf("%w: max duration must be positive", ErrValidation)
	}
	if c.RecordSize < 2 {
		return fmt.Errorf("%w: record size must be at least 2 bytes", ErrValidation)
	}
	if c.BufferSize <= 0 {
		return fmt.Errorf("%w: buffer size must be positive", ErrValidation)
	}
	if c.BatchSize <= 0 {
		return fmt.Errorf("%w: batch size must be positive", ErrValidation)
	}
	if c.QueueDepth < 0 {
		return fmt.Errorf("%w: queue depth cannot be negative", ErrValidation)
	}
	if !validWriter(c.Writer) {
		return fmt.Errorf("%w: unknown writer strategy %q", ErrValidation, c.Writer)
	}
	if !validProfile(c.Profile) {
		return fmt.Errorf("%w: unknown workload profile %q", ErrValidation, c.Profile)
	}
	if !validDurability(c.Durability) {
		return fmt.Errorf("%w: unknown durability mode %q", ErrValidation, c.Durability)
	}
	if !validOverwrite(c.Overwrite) {
		return fmt.Errorf("%w: unknown overwrite policy %q", ErrValidation, c.Overwrite)
	}
	if err := validateArtifactPath(c.OutputPath, "output"); err != nil {
		return err
	}
	if err := validateArtifactPath(c.ManifestPath, "manifest"); err != nil {
		return err
	}
	return nil
}

func validateArtifactPath(path, name string) error {
	if path == "" {
		return fmt.Errorf("%w: %s path is required", ErrValidation, name)
	}
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("%w: %s parent %q is not available: %v", ErrValidation, name, parent, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s parent %q is not a directory", ErrValidation, name, parent)
	}
	if info.Mode().Perm()&0200 == 0 {
		return fmt.Errorf("%w: %s parent %q is not writable", ErrValidation, name, parent)
	}
	return nil
}

func validWriter(v string) bool {
	return v == WriterDirect || v == WriterBuffered
}

func validProfile(v string) bool {
	switch v {
	case ProfileFixed, ProfileTiny, ProfileWide, ProfileHeavyTail, ProfileHighEntropy, ProfileLowEntropy, ProfileBurst:
		return true
	default:
		return false
	}
}

func validDurability(v string) bool {
	return v == DurabilityClosed || v == DurabilitySynced || v == DurabilityCommitted
}

func validOverwrite(v string) bool {
	return v == OverwriteFail || v == OverwriteReplace || v == OverwriteVersion
}

var (
	ErrValidation     = errors.New("validation")
	ErrFilesystem     = errors.New("filesystem")
	ErrTimeout        = errors.New("timeout")
	ErrResourceLimit  = errors.New("resource_limit")
	ErrApplicationBug = errors.New("application_bug")
)

func errorDomain(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrValidation):
		return "validation"
	case errors.Is(err, ErrFilesystem):
		return "filesystem"
	case errors.Is(err, ErrTimeout):
		return "timeout"
	case errors.Is(err, ErrResourceLimit):
		return "resource_limit"
	default:
		return "application_bug"
	}
}
