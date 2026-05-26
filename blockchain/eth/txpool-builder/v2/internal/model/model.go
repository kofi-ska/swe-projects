package model

import (
	"math/big"
	"time"
)

type ReasonCode string

const (
	ReasonConfigError          ReasonCode = "CONFIG_ERROR"
	ReasonRPCUnavailable       ReasonCode = "RPC_UNAVAILABLE"
	ReasonRPCSchemaError       ReasonCode = "RPC_SCHEMA_ERROR"
	ReasonRPCUnsupportedMethod ReasonCode = "RPC_UNSUPPORTED_METHOD"
	ReasonChainIDMismatch      ReasonCode = "CHAIN_ID_MISMATCH"
	ReasonSyncingNode          ReasonCode = "SYNCING_NODE"
	ReasonHeadDrift            ReasonCode = "HEAD_DRIFT"
	ReasonSnapshotTooLarge     ReasonCode = "SNAPSHOT_TOO_LARGE"
	ReasonDecodeError          ReasonCode = "DECODE_ERROR"
	ReasonMissingField         ReasonCode = "MISSING_FIELD"
	ReasonInvalidHex           ReasonCode = "INVALID_HEX"
	ReasonInvalidNonce         ReasonCode = "INVALID_NONCE"
	ReasonInvalidGas           ReasonCode = "INVALID_GAS"
	ReasonInvalidFeeModel      ReasonCode = "INVALID_FEE_MODEL"
	ReasonReplacementConflict  ReasonCode = "REPLACEMENT_CONFLICT"
	ReasonNonceGap             ReasonCode = "NONCE_GAP"
	ReasonExceedsBlockGas      ReasonCode = "EXCEEDS_BLOCK_GAS"
	ReasonCapacityExcluded     ReasonCode = "CAPACITY_EXCLUDED"
	ReasonPolicyRejected       ReasonCode = "POLICY_REJECTED"
	ReasonArtifactWriteFailed  ReasonCode = "ARTIFACT_WRITE_FAILED"
	ReasonTraceWriteFailed     ReasonCode = "TRACE_WRITE_FAILED"
	ReasonTimeout              ReasonCode = "TIMEOUT"
	ReasonInvariantFailure     ReasonCode = "INVARIANT_FAILURE"
)

type Config struct {
	ListenAddr         string
	RPCURL             string
	OutputDir          string
	PolicyVersion      string
	ChainID            *big.Int
	RefreshInterval    time.Duration
	RequestTimeout     time.Duration
	AdmissionTimeout   time.Duration
	QueueSize          int
	WorkerCount        int
	MaxRetainedJobs    int
	MaxRetainedSnap    int
	MaxArtifactBytes   int64
	MaxTraceBytes      int64
	MaxSnapshotBytes   int64
	MaxSnapshotAge     time.Duration
	MaxRPCPerRequest   int
	MaxRetryAttempts   int
	IncludeQueued      bool
	AllowHeadDrift     bool
	Strict             bool
	NoWrite            bool
	DryRunConfig       bool
	PrintConfig        bool
	Version            bool
	MaxGas             uint64
	MaxTransactions    int
	LowPriorityAllowed bool
}

// BuildRequest stays minimal so admission work remains cheap and bounded.
type BuildRequest struct {
	RequestID      string `json:"request_id,omitempty"`
	IdempotencyKey string `json:"idempotency_key"`
	PriorityClass  string `json:"priority_class"`
	PolicyVersion  string `json:"policy_version,omitempty"`
}

// BuildResponse returns early so clients do not wait on build execution.
type BuildResponse struct {
	RequestID    string     `json:"request_id"`
	JobID        string     `json:"job_id"`
	State        string     `json:"state"`
	ReasonCode   ReasonCode `json:"reason_code,omitempty"`
	ReasonDetail string     `json:"reason_detail,omitempty"`
	RetryAfterMS int64      `json:"retry_after_ms,omitempty"`
	SnapshotID   string     `json:"snapshot_id,omitempty"`
	ResultURI    string     `json:"result_uri,omitempty"`
	TraceURI     string     `json:"trace_uri,omitempty"`
}

type JobState string

const (
	JobReceived  JobState = "RECEIVED"
	JobQueued    JobState = "QUEUED"
	JobRunning   JobState = "RUNNING"
	JobCompleted JobState = "COMPLETED"
	JobFailed    JobState = "FAILED"
	JobShed      JobState = "SHED"
	JobDelayed   JobState = "DELAYED"
)

// RequestRecord keeps request identity and timing so replay stays auditable.
type RequestRecord struct {
	RequestID      string
	IdempotencyKey string
	PriorityClass  string
	PolicyVersion  string
	SubmittedAt    time.Time
}

// JobRecord keeps the small amount of mutable state needed to track one job.
type JobRecord struct {
	JobID          string
	RequestID      string
	IdempotencyKey string
	PriorityClass  string
	PolicyVersion  string
	State          JobState
	SnapshotID     string
	ReasonCode     ReasonCode
	ReasonDetail   string
	CreatedAt      time.Time
	StartedAt      time.Time
	CompletedAt    time.Time
	RetryAfterMS   int64
}

// Snapshot is immutable so one epoch can be reused safely across workers.
type Snapshot struct {
	SchemaVersion   int                      `json:"schema_version"`
	SnapshotID      string                   `json:"snapshot_id"`
	CapturedAt      time.Time                `json:"captured_at"`
	PolicyVersion   string                   `json:"policy_version"`
	BinaryVersion   string                   `json:"binary_version"`
	ChainID         string                   `json:"chain_id"`
	BaseFee         string                   `json:"base_fee,omitempty"`
	GasLimit        uint64                   `json:"gas_limit"`
	MempoolDigest   string                   `json:"mempool_digest"`
	FreshUntil      time.Time                `json:"fresh_until"`
	RefreshMS       int64                    `json:"refresh_ms"`
	PendingBySender map[string][]Transaction `json:"pending_by_sender,omitempty"`
	QueuedBySender  map[string][]Transaction `json:"queued_by_sender,omitempty"`
	SourceLabel     string                   `json:"source_label"`
	HeadBefore      string                   `json:"head_before"`
	HeadAfter       string                   `json:"head_after"`
	HeadDrift       bool                     `json:"head_drift"`
}

// Transaction is normalized so selection can stay deterministic and typed.
type Transaction struct {
	Hash                 string     `json:"hash"`
	From                 string     `json:"from"`
	To                   string     `json:"to,omitempty"`
	Nonce                uint64     `json:"nonce"`
	TxType               uint8      `json:"tx_type"`
	GasLimit             uint64     `json:"gas_limit"`
	MaxFeePerGas         *big.Int   `json:"max_fee_per_gas,omitempty"`
	MaxPriorityFeePerGas *big.Int   `json:"max_priority_fee_per_gas,omitempty"`
	GasPrice             *big.Int   `json:"gas_price,omitempty"`
	Value                *big.Int   `json:"value,omitempty"`
	InputLen             int        `json:"input_len"`
	EffectiveFee         *big.Int   `json:"effective_fee,omitempty"`
	Score                *big.Int   `json:"score,omitempty"`
	Eligible             bool       `json:"eligible"`
	ReasonCode           ReasonCode `json:"reason_code,omitempty"`
	ReasonDetail         string     `json:"reason_detail,omitempty"`
}

// Candidate is the persisted output because the service must be replayable.
type Candidate struct {
	SchemaVersion       int            `json:"schema_version"`
	CandidateID         string         `json:"candidate_id"`
	SnapshotID          string         `json:"snapshot_id"`
	PolicyVersion       string         `json:"policy_version"`
	BinaryVersion       string         `json:"binary_version"`
	ConfigDigest        string         `json:"config_digest,omitempty"`
	SourceEndpointLabel string         `json:"source_endpoint_label,omitempty"`
	ChainID             string         `json:"chain_id,omitempty"`
	SelectedTxs         []Transaction  `json:"selected_txs"`
	SelectedOrder       []string       `json:"selected_order"`
	TxCount             int            `json:"tx_count"`
	TotalGas            uint64         `json:"total_gas"`
	EstimatedRevenue    string         `json:"estimated_revenue,omitempty"`
	RejectedCount       int            `json:"rejected_count"`
	ReasonSummary       map[string]int `json:"reason_summary,omitempty"`
	SelectionStopReason string         `json:"selection_stop_reason,omitempty"`
	CreatedAt           time.Time      `json:"created_at"`
	TraceID             string         `json:"trace_id"`
	IsExecutableBlock   bool           `json:"is_executable_block"`
	BuildDurationMS     int64          `json:"build_duration_ms,omitempty"`
}

// Trace records decisions so load failures can be diagnosed after the fact.
type Trace struct {
	SchemaVersion       int            `json:"schema_version"`
	TraceID             string         `json:"trace_id"`
	SnapshotID          string         `json:"snapshot_id"`
	CandidateID         string         `json:"candidate_id"`
	PolicyVersion       string         `json:"policy_version"`
	BinaryVersion       string         `json:"binary_version"`
	ConfigDigest        string         `json:"config_digest,omitempty"`
	SourceEndpointLabel string         `json:"source_endpoint_label,omitempty"`
	ChainID             string         `json:"chain_id,omitempty"`
	ReasonCodeSummary   map[string]int `json:"reason_code_summary"`
	Accepted            []TxDecision   `json:"accepted,omitempty"`
	Rejected            []TxDecision   `json:"rejected,omitempty"`
	RankingOrder        []string       `json:"ranking_order,omitempty"`
	SelectionOrder      []string       `json:"selection_order,omitempty"`
	SelectionStopReason string         `json:"selection_stop_reason,omitempty"`
	CreatedAt           time.Time      `json:"created_at"`
	ReplayMode          bool           `json:"replay_mode"`
}

// TxDecision keeps one reason per tx so rejection accounting stays stable.
type TxDecision struct {
	TxHash        string     `json:"tx_hash"`
	From          string     `json:"from"`
	Nonce         uint64     `json:"nonce"`
	Accepted      bool       `json:"accepted"`
	PrimaryReason ReasonCode `json:"primary_reason_code,omitempty"`
	ReasonDetail  string     `json:"reason_detail,omitempty"`
	Stage         string     `json:"stage"`
	GasLimit      uint64     `json:"gas_limit,omitempty"`
	Score         string     `json:"score,omitempty"`
}

// Status exposes only the live counters needed for readiness and control.
type Status struct {
	Healthy         bool      `json:"healthy"`
	Mode            string    `json:"mode"`
	QueueDepth      int       `json:"queue_depth"`
	WorkerCount     int       `json:"worker_count"`
	SnapshotID      string    `json:"snapshot_id,omitempty"`
	SnapshotAgeMS   int64     `json:"snapshot_age_ms,omitempty"`
	LastRefreshMS   int64     `json:"last_refresh_ms,omitempty"`
	BuildsCompleted int64     `json:"builds_completed"`
	BuildsFailed    int64     `json:"builds_failed"`
	ShedCount       int64     `json:"shed_count"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Result ties the artifact paths to the job so retrieval stays O(1).
type Result struct {
	JobID        string     `json:"job_id"`
	RequestID    string     `json:"request_id"`
	SnapshotID   string     `json:"snapshot_id"`
	ArtifactURI  string     `json:"artifact_uri,omitempty"`
	TraceURI     string     `json:"trace_uri,omitempty"`
	Candidate    Candidate  `json:"candidate"`
	Trace        Trace      `json:"trace"`
	State        JobState   `json:"state"`
	ReasonCode   ReasonCode `json:"reason_code,omitempty"`
	ReasonDetail string     `json:"reason_detail,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  time.Time  `json:"completed_at"`
}

// StartupError keeps failures typed so callers can shed, retry, or abort.
type StartupError struct {
	Code   ReasonCode
	Stage  string
	Detail string
	Err    error
}

func (e *StartupError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err != nil {
		return string(e.Code) + ": " + e.Detail + ": " + e.Err.Error()
	}
	return string(e.Code) + ": " + e.Detail
}

// Unwrap lets callers recover the underlying error without losing the typed code.
func (e *StartupError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
