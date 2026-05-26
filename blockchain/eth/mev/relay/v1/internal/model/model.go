package model

import "time"

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Method  string          `json:"method"`
	Params  []BundleRequest `json:"params"`
}

type BundleRequest struct {
	Txs          []string `json:"txs"`
	BlockNumber  string   `json:"blockNumber"`
	MinTimestamp int64    `json:"minTimestamp"`
	MaxTimestamp int64    `json:"maxTimestamp"`
	Replacement  *string  `json:"replacementUuid,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// BundleState identifies a lifecycle state for a submitted bundle.
type BundleState string

const (
	StateReceived     BundleState = "received"
	StateValidated    BundleState = "validated"
	StateQueued       BundleState = "queued"
	StateSimulating   BundleState = "simulating"
	StateSimulated    BundleState = "simulated"
	StateScored       BundleState = "scored"
	StateRetryPending BundleState = "retry_pending"
	StateDeadLetter   BundleState = "dead_letter"
	StateForwarded    BundleState = "forwarded"
	StateRejected     BundleState = "rejected"
	StatePersisted    BundleState = "persisted"
	StateCompleted    BundleState = "completed"
)

type BundleRecord struct {
	ID          string        `json:"id"`
	BundleHash  string        `json:"bundleHash"`
	Request     BundleRequest `json:"request"`
	ClientID    string        `json:"clientId"`
	State       BundleState   `json:"state"`
	RetryCount  int           `json:"retryCount"`
	Score       float64       `json:"score"`
	ProfitEth   float64       `json:"profitEth"`
	Reason      string        `json:"reason"`
	Terminal    string        `json:"terminal"`
	Version     int64         `json:"version"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
	QueuedAt    time.Time     `json:"queuedAt,omitempty"`
	SimulatedAt time.Time     `json:"simulatedAt,omitempty"`
	CompletedAt time.Time     `json:"completedAt,omitempty"`
}

type EventRecord struct {
	Time     time.Time   `json:"time"`
	BundleID string      `json:"bundleId"`
	From     BundleState `json:"from,omitempty"`
	To       BundleState `json:"to"`
	Reason   string      `json:"reason,omitempty"`
	Version  int64       `json:"version"`
	ClientID string      `json:"clientId,omitempty"`
}

type SimulationResult struct {
	ProfitEth float64
	LatencyMS int64
	Success   bool
	Reason    string
}

type Decision struct {
	Action    string  `json:"action"`
	Reason    string  `json:"reason"`
	Score     float64 `json:"score"`
	ProfitEth float64 `json:"profitEth"`
	BlockHash string  `json:"blockHash,omitempty"`
}
