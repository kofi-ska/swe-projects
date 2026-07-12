package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunWritesManifestAndFinalArtifact(t *testing.T) {
	dir := t.TempDir()
	cfg := testRunConfig(dir, WriterBuffered, ProfileFixed, 16)

	result, err := Run(cfg)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if result.ArtifactState != "final" {
		t.Fatalf("artifact state = %q, want final", result.ArtifactState)
	}
	if result.DurationFlushCloseNanos <= 0 {
		t.Fatalf("flush/close duration was not recorded")
	}
	if _, err := os.Stat(cfg.OutputPath); err != nil {
		t.Fatalf("output artifact not written: %v", err)
	}

	payload, err := os.ReadFile(cfg.ManifestPath)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	var manifest RunResult
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if manifest.SchemaVersion != manifestSchemaVersion {
		t.Fatalf("manifest schema = %q, want %q", manifest.SchemaVersion, manifestSchemaVersion)
	}
}

func TestDirectAndBufferedWriteSamePayload(t *testing.T) {
	dir := t.TempDir()
	direct := testRunConfig(filepath.Join(dir, "direct"), WriterDirect, ProfileHighEntropy, 64)
	buffered := testRunConfig(filepath.Join(dir, "buffered"), WriterBuffered, ProfileHighEntropy, 64)

	if _, err := Run(direct); err != nil {
		t.Fatalf("direct Run() error = %v", err)
	}
	if _, err := Run(buffered); err != nil {
		t.Fatalf("buffered Run() error = %v", err)
	}

	directBytes, err := os.ReadFile(direct.OutputPath)
	if err != nil {
		t.Fatalf("read direct output: %v", err)
	}
	bufferedBytes, err := os.ReadFile(buffered.OutputPath)
	if err != nil {
		t.Fatalf("read buffered output: %v", err)
	}
	if string(directBytes) != string(bufferedBytes) {
		t.Fatalf("direct and buffered outputs differ")
	}
}

func TestValidationRejectsMissingArtifactParent(t *testing.T) {
	cfg := DefaultRunConfig()
	cfg.OutputPath = filepath.Join(t.TempDir(), "missing", "out.txt")
	cfg.ManifestPath = filepath.Join(t.TempDir(), "manifest.json")

	result, err := Run(cfg)
	if err == nil {
		t.Fatalf("Run() error = nil, want validation error")
	}
	if result.Status != "rejected" {
		t.Fatalf("status = %q, want rejected", result.Status)
	}
	if result.ErrorDomain != "validation" {
		t.Fatalf("error domain = %q, want validation", result.ErrorDomain)
	}
}

func TestGeneratorIsDeterministic(t *testing.T) {
	left := NewRecordGenerator(ProfileHeavyTail, 42, 32)
	right := NewRecordGenerator(ProfileHeavyTail, 42, 32)

	for i := 0; i < 1000; i++ {
		if string(left.Next()) != string(right.Next()) {
			t.Fatalf("record %d differs for same seed", i)
		}
	}
}

func testRunConfig(dir, writer, profile string, recordSize int) RunConfig {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}
	return RunConfig{
		RecordsLimit: 512,
		MaxBytes:     4 * 1024 * 1024,
		MaxDuration:  5 * time.Second,
		Writer:       writer,
		Profile:      profile,
		Seed:         99,
		RecordSize:   recordSize,
		BufferSize:   16 * 1024,
		BatchSize:    64 * 1024,
		OutputPath:   filepath.Join(dir, "out.txt"),
		ManifestPath: filepath.Join(dir, "manifest.json"),
		Durability:   DurabilityClosed,
		Overwrite:    OverwriteReplace,
	}
}
