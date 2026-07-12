package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

const manifestSchemaVersion = "multiwrite.v1.run.1"

type RunResult struct {
	SchemaVersion           string         `json:"schema_version"`
	RunID                   string         `json:"run_id"`
	BinaryVersion           string         `json:"binary_version"`
	StartedAt               time.Time      `json:"started_at"`
	EndedAt                 time.Time      `json:"ended_at"`
	Status                  string         `json:"status"`
	ErrorDomain             string         `json:"error_domain,omitempty"`
	Error                   string         `json:"error,omitempty"`
	Config                  RunConfig      `json:"config"`
	RecordsRequested        int64          `json:"records_requested"`
	RecordsGenerated        int64          `json:"records_generated"`
	RecordsWritten          int64          `json:"records_written"`
	BytesWritten            int64          `json:"bytes_written"`
	DurationTotalNanos      int64          `json:"duration_total_nanos"`
	DurationWriteLoopNanos  int64          `json:"duration_write_loop_nanos"`
	DurationFlushCloseNanos int64          `json:"duration_flush_close_nanos"`
	DurationCommitNanos     int64          `json:"duration_commit_nanos"`
	RecordsPerSecond        float64        `json:"records_per_second"`
	MiBPerSecond            float64        `json:"mib_per_second"`
	ArtifactState           string         `json:"artifact_state"`
	OutputPath              string         `json:"output_path"`
	TempPath                string         `json:"temp_path,omitempty"`
	ManifestPath            string         `json:"manifest_path"`
	Environment             RunEnvironment `json:"environment"`
}

type RunEnvironment struct {
	Class        string `json:"class"`
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	GoVersion    string `json:"go_version"`
	GoMaxProcs   int    `json:"gomaxprocs"`
	GoMemLimit   string `json:"gomemlimit,omitempty"`
	CPULimit     string `json:"cpu_limit,omitempty"`
	MemoryLimit  string `json:"memory_limit,omitempty"`
	Filesystem   string `json:"filesystem,omitempty"`
	VolumeKind   string `json:"volume_kind,omitempty"`
	Image        string `json:"container_image,omitempty"`
	StorageClass string `json:"storage_class,omitempty"`
	PVCSize      string `json:"pvc_size,omitempty"`
	NodeName     string `json:"node_name,omitempty"`
}

func Run(cfg RunConfig) (RunResult, error) {
	cfg.ApplyDefaults()

	start := time.Now()
	result := RunResult{
		SchemaVersion:    manifestSchemaVersion,
		RunID:            makeRunID(start),
		BinaryVersion:    envDefault("MULTIWRITE_VERSION", "dev"),
		StartedAt:        start.UTC(),
		Status:           "created",
		Config:           cfg,
		RecordsRequested: cfg.RecordsLimit,
		ArtifactState:    "none",
		ManifestPath:     cfg.ManifestPath,
		Environment:      detectEnvironment(),
	}

	if err := cfg.Validate(); err != nil {
		finishResult(&result, "rejected", err, start)
		return result, err
	}

	finalPath, err := resolveOutputPath(cfg)
	if err != nil {
		finishResult(&result, "rejected", err, start)
		_ = writeManifest(result)
		return result, err
	}
	tempPath := finalPath + ".tmp"
	result.OutputPath = finalPath
	result.TempPath = tempPath

	if _, err := os.Stat(tempPath); err == nil {
		runErr := fmt.Errorf("%w: temp artifact already exists: %s", ErrValidation, tempPath)
		finishResult(&result, "rejected", runErr, start)
		_ = writeManifest(result)
		return result, runErr
	} else if !os.IsNotExist(err) {
		runErr := fmt.Errorf("%w: inspect temp artifact: %v", ErrFilesystem, err)
		finishResult(&result, "rejected", runErr, start)
		_ = writeManifest(result)
		return result, runErr
	}

	writer, err := newMeasuredWriter(tempPath, cfg)
	if err != nil {
		finishResult(&result, "failed", err, start)
		_ = writeManifest(result)
		return result, err
	}
	result.ArtifactState = "temp"

	generator := NewRecordGenerator(cfg.Profile, cfg.Seed, cfg.RecordSize)
	writeStart := time.Now()
	result.Status = "running"

	for result.RecordsGenerated < cfg.RecordsLimit {
		if time.Since(start) > cfg.MaxDuration {
			err = fmt.Errorf("%w: run exceeded %s", ErrTimeout, cfg.MaxDuration)
			break
		}

		record := generator.Next()
		nextBytes := result.BytesWritten + int64(len(record))
		if nextBytes > cfg.MaxBytes {
			break
		}

		result.RecordsGenerated++
		if writeErr := writer.WriteRecord(record); writeErr != nil {
			err = writeErr
			break
		}
		result.RecordsWritten++
		result.BytesWritten = nextBytes
	}
	result.DurationWriteLoopNanos = time.Since(writeStart).Nanoseconds()

	if err != nil {
		closeErr := writer.Close()
		if closeErr != nil {
			err = fmt.Errorf("%v; close after failure: %w", err, closeErr)
		}
		if result.BytesWritten > 0 {
			result.ArtifactState = "partial"
			finishResult(&result, "partial", err, start)
		} else {
			finishResult(&result, "failed", err, start)
		}
		_ = writeManifest(result)
		return result, err
	}

	flushStart := time.Now()
	if err = writer.Flush(); err == nil && needsSync(cfg.Durability) {
		err = writer.Sync()
	}
	if closeErr := writer.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	result.DurationFlushCloseNanos = time.Since(flushStart).Nanoseconds()
	if err != nil {
		result.ArtifactState = "partial"
		finishResult(&result, "partial", err, start)
		_ = writeManifest(result)
		return result, err
	}

	commitStart := time.Now()
	if err = os.Rename(tempPath, finalPath); err != nil {
		runErr := fmt.Errorf("%w: commit artifact: %v", ErrFilesystem, err)
		result.ArtifactState = "partial"
		result.DurationCommitNanos = time.Since(commitStart).Nanoseconds()
		finishResult(&result, "partial", runErr, start)
		_ = writeManifest(result)
		return result, runErr
	}
	result.DurationCommitNanos = time.Since(commitStart).Nanoseconds()
	result.ArtifactState = "final"

	finishResult(&result, "completed", nil, start)
	if err = writeManifest(result); err != nil {
		return result, err
	}
	return result, nil
}

func finishResult(result *RunResult, status string, err error, start time.Time) {
	result.Status = status
	result.EndedAt = time.Now().UTC()
	result.DurationTotalNanos = time.Since(start).Nanoseconds()
	if err != nil {
		result.ErrorDomain = errorDomain(err)
		result.Error = err.Error()
	}
	seconds := float64(result.DurationTotalNanos) / float64(time.Second)
	if seconds > 0 {
		result.RecordsPerSecond = float64(result.RecordsWritten) / seconds
		result.MiBPerSecond = (float64(result.BytesWritten) / (1024 * 1024)) / seconds
	}
}

func resolveOutputPath(cfg RunConfig) (string, error) {
	if _, err := os.Stat(cfg.OutputPath); err == nil {
		switch cfg.Overwrite {
		case OverwriteFail:
			return "", fmt.Errorf("%w: output already exists: %s", ErrValidation, cfg.OutputPath)
		case OverwriteVersion:
			return versionedPath(cfg.OutputPath), nil
		case OverwriteReplace:
			return cfg.OutputPath, nil
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("%w: inspect output: %v", ErrFilesystem, err)
	}
	return cfg.OutputPath, nil
}

func versionedPath(path string) string {
	ext := filepath.Ext(path)
	base := path[:len(path)-len(ext)]
	return base + "." + strconv.FormatInt(time.Now().UTC().UnixNano(), 10) + ext
}

func writeManifest(result RunResult) error {
	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: marshal manifest: %v", ErrApplicationBug, err)
	}
	if err := os.WriteFile(result.ManifestPath, append(payload, '\n'), 0o644); err != nil {
		return fmt.Errorf("%w: write manifest: %v", ErrFilesystem, err)
	}
	return nil
}

func needsSync(durability string) bool {
	return durability == DurabilitySynced || durability == DurabilityCommitted
}

func makeRunID(t time.Time) string {
	return "run-" + strconv.FormatInt(t.UTC().UnixNano(), 10)
}

func detectEnvironment() RunEnvironment {
	return RunEnvironment{
		Class:        envDefault("MULTIWRITE_ENV", "local"),
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		GoVersion:    runtime.Version(),
		GoMaxProcs:   runtime.GOMAXPROCS(0),
		GoMemLimit:   os.Getenv("GOMEMLIMIT"),
		CPULimit:     os.Getenv("MULTIWRITE_CPU_LIMIT"),
		MemoryLimit:  os.Getenv("MULTIWRITE_MEMORY_LIMIT"),
		Filesystem:   os.Getenv("MULTIWRITE_FILESYSTEM"),
		VolumeKind:   os.Getenv("MULTIWRITE_VOLUME_KIND"),
		Image:        os.Getenv("MULTIWRITE_IMAGE"),
		StorageClass: os.Getenv("MULTIWRITE_STORAGE_CLASS"),
		PVCSize:      os.Getenv("MULTIWRITE_PVC_SIZE"),
		NodeName:     os.Getenv("NODE_NAME"),
	}
}

func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
