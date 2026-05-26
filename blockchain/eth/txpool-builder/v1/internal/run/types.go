package run

import (
	"encoding/json"
	"time"

	"txpool-builder/v1/internal/model"
)

// BinaryVersion is embedded so replay and artifact identity stay reproducible.
var BinaryVersion = "v1-dev"

// Result ties the build outputs together so callers can persist or compare them as one unit.
type Result struct {
	ConfigDigest string
	Candidate    model.BlockCandidate
	Trace        model.DecisionTrace
	Snapshot     canonicalSnapshot
	Replay       bool
	Comparison   *ComparisonResult
}

// ComparisonResult reports replay drift without forcing a full diff at call sites.
type ComparisonResult struct {
	Match       bool     `json:"match"`
	Differences []string `json:"differences,omitempty"`
}

// canonicalSnapshot stores the stable snapshot form so replay can stay deterministic.
type canonicalSnapshot struct {
	SchemaVersion       int                 `json:"schema_version"`
	SnapshotID          string              `json:"snapshot_id"`
	SourceEndpointLabel string              `json:"source_endpoint_label"`
	CapturedAt          time.Time           `json:"captured_at"`
	HeadBefore          string              `json:"head_before"`
	HeadAfter           string              `json:"head_after"`
	HeadDrift           bool                `json:"head_drift"`
	ChainID             string              `json:"chain_id"`
	ClientVersion       string              `json:"client_version,omitempty"`
	FetchDurationMS     int64               `json:"fetch_duration_ms"`
	RawPayloadDigest    string              `json:"raw_payload_digest"`
	RawPendingCount     int                 `json:"raw_pending_count"`
	RawQueuedCount      int                 `json:"raw_queued_count"`
	Pending             []model.Transaction `json:"pending"`
	Queued              []model.Transaction `json:"queued,omitempty"`
	RawSnapshotPath     string              `json:"raw_snapshot_path,omitempty"`
	RawSnapshotSize     int64               `json:"raw_snapshot_size_bytes"`
}

// snapshotFingerprint narrows the hash input to the fields that define the epoch.
type snapshotFingerprint struct {
	SchemaVersion       int                 `json:"schema_version"`
	SourceEndpointLabel string              `json:"source_endpoint_label"`
	HeadBefore          string              `json:"head_before"`
	HeadAfter           string              `json:"head_after"`
	HeadDrift           bool                `json:"head_drift"`
	ChainID             string              `json:"chain_id"`
	ClientVersion       string              `json:"client_version,omitempty"`
	RawPayloadDigest    string              `json:"raw_payload_digest"`
	RawPendingCount     int                 `json:"raw_pending_count"`
	RawQueuedCount      int                 `json:"raw_queued_count"`
	Pending             []model.Transaction `json:"pending"`
	Queued              []model.Transaction `json:"queued,omitempty"`
	RawSnapshotSize     int64               `json:"raw_snapshot_size_bytes"`
}

// rawPool mirrors txpool_content so malformed payloads can be rejected early.
type rawPool struct {
	Pending map[string]map[string]json.RawMessage `json:"pending"`
	Queued  map[string]map[string]json.RawMessage `json:"queued"`
}

// senderGroup keeps per-sender candidates together before global ranking.
type senderGroup struct {
	Sender string
	Txs    []model.Transaction
}

// decisionBuckets separates decode, validation, policy, capacity, and acceptance counts.
type decisionBuckets struct {
	DecodeFailures     []model.TxDecision
	ValidationFailures []model.TxDecision
	PolicyRejections   []model.TxDecision
	CapacityExclusions []model.TxDecision
	Accepted           []model.TxDecision
}

// senderNormalization keeps sender-chain rejection reasons separate from global selection.
type senderNormalization struct {
	PolicyRejections   []model.TxDecision
	ValidationFailures []model.TxDecision
}

// selectionResult keeps the selection outcome small but auditable.
type selectionResult struct {
	Accepted           []model.TxDecision
	CapacityExclusions []model.TxDecision
	ReasonSummary      map[string]int
}
