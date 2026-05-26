package model

import (
	"math/big"
	"time"
)

type ReasonCode string

const (
	ReasonConfigError          ReasonCode = "CONFIG_ERROR"
	ReasonRPCUnavailable       ReasonCode = "RPC_UNAVAILABLE"
	ReasonRPCTimeout           ReasonCode = "RPC_TIMEOUT"
	ReasonRPCSchemaError       ReasonCode = "RPC_SCHEMA_ERROR"
	ReasonRPCUnsupportedMethod ReasonCode = "RPC_UNSUPPORTED_METHOD"
	ReasonChainIDMismatch      ReasonCode = "CHAIN_ID_MISMATCH"
	ReasonSyncingNode          ReasonCode = "SYNCING_NODE"
	ReasonSnapshotTooLarge     ReasonCode = "SNAPSHOT_TOO_LARGE"
	ReasonHeadDrift            ReasonCode = "HEAD_DRIFT"
	ReasonArtifactWriteFailed  ReasonCode = "ARTIFACT_WRITE_FAILED"
	ReasonTraceWriteFailed     ReasonCode = "TRACE_WRITE_FAILED"
	ReasonInvariantFailure     ReasonCode = "INVARIANT_FAILURE"
	ReasonTimeout              ReasonCode = "TIMEOUT"
	ReasonPanicRecovered       ReasonCode = "PANIC_RECOVERED"

	ReasonDecodeError              ReasonCode = "DECODE_ERROR"
	ReasonUnsupportedTxType        ReasonCode = "UNSUPPORTED_TX_TYPE"
	ReasonMissingField             ReasonCode = "MISSING_FIELD"
	ReasonInvalidHex               ReasonCode = "INVALID_HEX"
	ReasonOverflow                 ReasonCode = "OVERFLOW"
	ReasonInvalidAddress           ReasonCode = "INVALID_ADDRESS"
	ReasonInvalidNonce             ReasonCode = "INVALID_NONCE"
	ReasonNonceGap                 ReasonCode = "NONCE_GAP"
	ReasonDuplicateNonce           ReasonCode = "DUPLICATE_NONCE"
	ReasonDuplicateTx              ReasonCode = "DUPLICATE_TX"
	ReasonReplacementConflict      ReasonCode = "REPLACEMENT_CONFLICT"
	ReasonInvalidGasLimit          ReasonCode = "INVALID_GAS_LIMIT"
	ReasonExceedsBlockGas          ReasonCode = "EXCEEDS_BLOCK_GAS"
	ReasonInvalidFeeModel          ReasonCode = "INVALID_FEE_MODEL"
	ReasonInsufficientEffectiveFee ReasonCode = "INSUFFICIENT_EFFECTIVE_FEE"
	ReasonPolicyRejected           ReasonCode = "POLICY_REJECTED"
	ReasonCapacityExcluded         ReasonCode = "CAPACITY_EXCLUDED"
	ReasonStaleSnapshot            ReasonCode = "STALE_SNAPSHOT"
)

type Config struct {
	ConfigPath            string
	RPCURL                string
	OutputPath            string
	TraceOutputPath       string
	SnapshotOutputPath    string
	ReplaySnapshotPath    string
	CompareCandidatePath  string
	Timeout               time.Duration
	MaxTransactions       int
	MaxGas                uint64
	MaxSnapshotTxs        int
	MaxRawSnapshotBytes   int64
	MaxArtifactBytes      int64
	MaxTraceBytes         int64
	PolicyVersion         string
	ChainID               *big.Int
	Strict                bool
	RejectOnPartialDecode bool
	AllowHeadDrift        bool
	IncludeQueued         bool
	NoWrite               bool
	DryRunConfig          bool
	PrintConfig           bool
	Version               bool
}

type StartupInfo struct {
	EndpointLabel string
	ChainID       *big.Int
	ClientVersion string
	BlockNumber   *big.Int
	Syncing       bool
}

type Snapshot struct {
	SchemaVersion       int       `json:"schema_version"`
	SnapshotID          string    `json:"snapshot_id"`
	SourceEndpointLabel string    `json:"source_endpoint_label"`
	CapturedAt          time.Time `json:"captured_at"`
	HeadBefore          string    `json:"head_before"`
	HeadAfter           string    `json:"head_after"`
	HeadDrift           bool      `json:"head_drift"`
	ChainID             string    `json:"chain_id"`
	ClientVersion       string    `json:"client_version,omitempty"`
	FetchDurationMS     int64     `json:"fetch_duration_ms"`
	RawPayloadDigest    string    `json:"raw_payload_digest"`
	RawPendingCount     int       `json:"raw_pending_count"`
	RawQueuedCount      int       `json:"raw_queued_count"`
	Pending             []RawTx   `json:"pending"`
	Queued              []RawTx   `json:"queued,omitempty"`
	RawSnapshotPath     string    `json:"raw_snapshot_path,omitempty"`
	RawSnapshotSize     int64     `json:"raw_snapshot_size_bytes"`
	Source              string    `json:"source,omitempty"`
}

type RawTx struct {
	Sender  string
	Nonce   uint64
	Hash    string
	Raw     map[string]any
	RawJSON []byte
	Source  string
}

type Transaction struct {
	Hash                 string         `json:"hash"`
	From                 string         `json:"from"`
	To                   *string        `json:"to,omitempty"`
	Nonce                uint64         `json:"nonce"`
	TxType               uint8          `json:"tx_type"`
	GasLimit             uint64         `json:"gas_limit"`
	MaxFeePerGas         *big.Int       `json:"max_fee_per_gas,omitempty"`
	MaxPriorityFeePerGas *big.Int       `json:"max_priority_fee_per_gas,omitempty"`
	GasPrice             *big.Int       `json:"gas_price,omitempty"`
	Value                *big.Int       `json:"value"`
	InputLen             int            `json:"input_len"`
	AccessList           int            `json:"access_list"`
	RawMetadata          map[string]any `json:"raw_metadata,omitempty"`
	IntrinsicGas         uint64         `json:"intrinsic_gas"`
	Eligible             bool           `json:"eligible"`
	ReasonCode           ReasonCode     `json:"reason_code,omitempty"`
	ReasonDetail         string         `json:"reason_detail,omitempty"`
	EffectivePriorityFee *big.Int       `json:"effective_priority_fee,omitempty"`
	EffectiveGasPrice    *big.Int       `json:"effective_gas_price,omitempty"`
	Score                *big.Int       `json:"score,omitempty"`
	SenderNonceGroupSize int            `json:"sender_nonce_group_size,omitempty"`
}

type ValidationResult struct {
	TxHash       string     `json:"tx_hash"`
	Accepted     bool       `json:"accepted"`
	ReasonCode   ReasonCode `json:"reason_code"`
	ReasonDetail string     `json:"reason_detail,omitempty"`
	Stage        string     `json:"stage"`
	RankKey      string     `json:"rank_key,omitempty"`
}

type TxDecision struct {
	TxHash         string     `json:"tx_hash"`
	From           string     `json:"from"`
	Nonce          uint64     `json:"nonce"`
	Accepted       bool       `json:"accepted"`
	PrimaryReason  ReasonCode `json:"primary_reason_code,omitempty"`
	ReasonDetail   string     `json:"reason_detail,omitempty"`
	Stage          string     `json:"stage"`
	RankPosition   int        `json:"rank_position,omitempty"`
	SelectionPos   int        `json:"selection_position,omitempty"`
	GasLimit       uint64     `json:"gas_limit"`
	Score          string     `json:"score,omitempty"`
	EffectiveTip   string     `json:"effective_tip,omitempty"`
	EffectivePrice string     `json:"effective_gas_price,omitempty"`
}

type BlockCandidate struct {
	SchemaVersion            int            `json:"schema_version"`
	CandidateID              string         `json:"candidate_id"`
	SnapshotID               string         `json:"snapshot_id"`
	PolicyVersion            string         `json:"policy_version"`
	BinaryVersion            string         `json:"binary_version"`
	ConfigDigest             string         `json:"config_digest"`
	SourceEndpointLabel      string         `json:"source_endpoint_label"`
	ChainID                  string         `json:"chain_id"`
	SelectedTxs              []Transaction  `json:"selected_txs"`
	SelectedTxHashes         []string       `json:"selected_tx_hashes"`
	SelectedOrder            []string       `json:"selected_order"`
	TxCount                  int            `json:"tx_count"`
	TotalGas                 uint64         `json:"total_gas"`
	EstimatedPriorityRevenue string         `json:"estimated_priority_revenue"`
	RejectedCount            int            `json:"rejected_count"`
	RejectionSummary         map[string]int `json:"rejection_summary"`
	SelectionStopReason      string         `json:"selection_stop_reason"`
	BuildDurationMS          int64          `json:"build_duration_ms"`
	CreatedAt                time.Time      `json:"created_at"`
	TraceRef                 string         `json:"trace_ref"`
	IsExecutableBlock        bool           `json:"is_executable_block"`
}

type DecisionTrace struct {
	SchemaVersion       int            `json:"schema_version"`
	TraceID             string         `json:"trace_id"`
	SnapshotID          string         `json:"snapshot_id"`
	CandidateID         string         `json:"candidate_id"`
	PolicyVersion       string         `json:"policy_version"`
	BinaryVersion       string         `json:"binary_version"`
	ConfigDigest        string         `json:"config_digest"`
	SourceEndpointLabel string         `json:"source_endpoint_label"`
	ChainID             string         `json:"chain_id"`
	DecodeFailures      []TxDecision   `json:"decode_failures,omitempty"`
	ValidationFailures  []TxDecision   `json:"validation_failures,omitempty"`
	PolicyRejections    []TxDecision   `json:"policy_rejections,omitempty"`
	CapacityExclusions  []TxDecision   `json:"capacity_exclusions,omitempty"`
	Accepted            []TxDecision   `json:"accepted,omitempty"`
	RankingOrder        []string       `json:"ranking_order"`
	SelectionOrder      []string       `json:"selection_order"`
	ReasonCodeSummary   map[string]int `json:"reason_code_summary"`
	SelectionStopReason string         `json:"selection_stop_reason"`
	FinalSummary        string         `json:"final_summary"`
	CreatedAt           time.Time      `json:"created_at"`
	ReplayMode          bool           `json:"replay_mode"`
}

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

func (e *StartupError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
