package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	multiwrite "multiwrite/src"
)

func TestRunWritesManifestAndFinalArtifact(t *testing.T) {
	dir := t.TempDir()
	cfg := testRunConfig(dir, multiwrite.WriterBuffered, multiwrite.ProfileFixed, 16)

	result, err := multiwrite.Run(cfg)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if result.ArtifactState != "final" {
		t.Fatalf("artifact state = %q, want final", result.ArtifactState)
	}
	if result.TerminalReason != multiwrite.ReasonRecordsExhausted {
		t.Fatalf("terminal reason = %q, want %q", result.TerminalReason, multiwrite.ReasonRecordsExhausted)
	}
	if result.DurationFlushCloseNanos <= 0 {
		t.Fatalf("flush/close duration was not recorded")
	}
	if result.DurationManifestNanos <= 0 {
		t.Fatalf("manifest duration was not recorded in returned result")
	}
	if _, err := os.Stat(cfg.OutputPath); err != nil {
		t.Fatalf("output artifact not written: %v", err)
	}

	payload, err := os.ReadFile(cfg.ManifestPath)
	if err != nil {
		t.Fatalf("manifest not written: %v", err)
	}
	var manifest multiwrite.RunResult
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}
	if manifest.SchemaVersion != multiwrite.ManifestSchemaVersion {
		t.Fatalf("manifest schema = %q, want %q", manifest.SchemaVersion, multiwrite.ManifestSchemaVersion)
	}
}

func TestDirectAndBufferedWriteSamePayload(t *testing.T) {
	dir := t.TempDir()
	direct := testRunConfig(filepath.Join(dir, "direct"), multiwrite.WriterDirect, multiwrite.ProfileHighEntropy, 64)
	buffered := testRunConfig(filepath.Join(dir, "buffered"), multiwrite.WriterBuffered, multiwrite.ProfileHighEntropy, 64)

	if _, err := multiwrite.Run(direct); err != nil {
		t.Fatalf("direct Run() error = %v", err)
	}
	if _, err := multiwrite.Run(buffered); err != nil {
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

func TestRunRecordsByteLimitTerminalReason(t *testing.T) {
	dir := t.TempDir()
	cfg := testRunConfig(dir, multiwrite.WriterBuffered, multiwrite.ProfileFixed, 64)
	cfg.RecordsLimit = 1000
	cfg.MaxBytes = 100

	result, err := multiwrite.Run(cfg)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if result.TerminalReason != multiwrite.ReasonByteLimitReached {
		t.Fatalf("terminal reason = %q, want %q", result.TerminalReason, multiwrite.ReasonByteLimitReached)
	}
	if result.RecordsWritten >= cfg.RecordsLimit {
		t.Fatalf("records written = %d, want fewer than %d", result.RecordsWritten, cfg.RecordsLimit)
	}
}

func TestValidationRejectsOutputManifestCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "same-path")
	cfg := testRunConfig(dir, multiwrite.WriterBuffered, multiwrite.ProfileFixed, 16)
	cfg.OutputPath = path
	cfg.ManifestPath = path

	result, err := multiwrite.Run(cfg)
	if err == nil {
		t.Fatalf("Run() error = nil, want validation error")
	}
	if result.Status != "rejected" {
		t.Fatalf("status = %q, want rejected", result.Status)
	}
	if result.TerminalReason != multiwrite.ReasonValidationFailed {
		t.Fatalf("terminal reason = %q, want %q", result.TerminalReason, multiwrite.ReasonValidationFailed)
	}
}

func TestValidationRejectsUnusedControls(t *testing.T) {
	dir := t.TempDir()
	cfg := testRunConfig(dir, multiwrite.WriterBuffered, multiwrite.ProfileFixed, 16)
	cfg.QueueDepth = 1

	result, err := multiwrite.Run(cfg)
	if err == nil {
		t.Fatalf("Run() error = nil, want validation error")
	}
	if result.ErrorDomain != "validation" {
		t.Fatalf("error domain = %q, want validation", result.ErrorDomain)
	}

	cfg = testRunConfig(dir, multiwrite.WriterBuffered, multiwrite.ProfileFixed, 16)
	cfg.BatchSize = multiwrite.DefaultBatchSize * 2
	result, err = multiwrite.Run(cfg)
	if err == nil {
		t.Fatalf("Run() error = nil, want validation error")
	}
	if result.ErrorDomain != "validation" {
		t.Fatalf("error domain = %q, want validation", result.ErrorDomain)
	}
}

func TestRunRejectsExistingTempArtifact(t *testing.T) {
	dir := t.TempDir()
	cfg := testRunConfig(dir, multiwrite.WriterBuffered, multiwrite.ProfileFixed, 16)
	if err := os.WriteFile(cfg.OutputPath+".tmp", []byte("stale"), 0o644); err != nil {
		t.Fatalf("write temp artifact: %v", err)
	}

	result, err := multiwrite.Run(cfg)
	if err == nil {
		t.Fatalf("Run() error = nil, want validation error")
	}
	if result.Status != "rejected" {
		t.Fatalf("status = %q, want rejected", result.Status)
	}
	if result.TerminalReason != multiwrite.ReasonValidationFailed {
		t.Fatalf("terminal reason = %q, want %q", result.TerminalReason, multiwrite.ReasonValidationFailed)
	}
}

func TestValidationRejectsMissingArtifactParent(t *testing.T) {
	cfg := multiwrite.DefaultRunConfig()
	cfg.OutputPath = filepath.Join(t.TempDir(), "missing", "out.txt")
	cfg.ManifestPath = filepath.Join(t.TempDir(), "manifest.json")

	result, err := multiwrite.Run(cfg)
	if err == nil {
		t.Fatalf("Run() error = nil, want validation error")
	}
	if result.Status != "rejected" {
		t.Fatalf("status = %q, want rejected", result.Status)
	}
	if result.TerminalReason != multiwrite.ReasonValidationFailed {
		t.Fatalf("terminal reason = %q, want %q", result.TerminalReason, multiwrite.ReasonValidationFailed)
	}
	if result.ErrorDomain != "validation" {
		t.Fatalf("error domain = %q, want validation", result.ErrorDomain)
	}
}

func TestGeneratorIsDeterministic(t *testing.T) {
	left := multiwrite.NewRecordGenerator(multiwrite.ProfileHeavyTail, 42, 32)
	right := multiwrite.NewRecordGenerator(multiwrite.ProfileHeavyTail, 42, 32)

	for i := 0; i < 1000; i++ {
		if string(left.Next()) != string(right.Next()) {
			t.Fatalf("record %d differs for same seed", i)
		}
	}
}

func testRunConfig(dir, writer, profile string, recordSize int) multiwrite.RunConfig {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		panic(err)
	}
	return multiwrite.RunConfig{
		RecordsLimit: 512,
		MaxBytes:     4 * 1024 * 1024,
		MaxDuration:  5 * time.Second,
		Writer:       writer,
		Profile:      profile,
		Seed:         99,
		RecordSize:   recordSize,
		BufferSize:   16 * 1024,
		BatchSize:    multiwrite.DefaultBatchSize,
		OutputPath:   filepath.Join(dir, "out.txt"),
		ManifestPath: filepath.Join(dir, "manifest.json"),
		Durability:   multiwrite.DurabilityClosed,
		Overwrite:    multiwrite.OverwriteReplace,
	}
}
