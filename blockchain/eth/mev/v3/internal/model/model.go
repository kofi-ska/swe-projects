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
	ID                string        `json:"id"`
	BundleHash        string        `json:"bundleHash"`
	Request           BundleRequest `json:"request"`
	ClientID          string        `json:"clientId"`
	RegionID          string        `json:"regionId"`
	ShardID           string        `json:"shardId"`
	LeaseID           string        `json:"leaseId"`
	LeaseEpoch        uint64        `json:"leaseEpoch"`
	FenceToken        uint64        `json:"fenceToken"`
	State             BundleState   `json:"state"`
	RetryCount        int           `json:"retryCount"`
	Score             float64       `json:"score"`
	ProfitEth         float64       `json:"profitEth"`
	Reason            string        `json:"reason"`
	Terminal          string        `json:"terminal"`
	Version           int64         `json:"version"`
	Sequence          uint64        `json:"sequence"`
	CreatedAt         time.Time     `json:"createdAt"`
	UpdatedAt         time.Time     `json:"updatedAt"`
	QueuedAt          time.Time     `json:"queuedAt,omitempty"`
	CompletedAt       time.Time     `json:"completedAt,omitempty"`
	DeadlineAt        time.Time     `json:"deadlineAt,omitempty"`
	ExpectedValue     float64       `json:"expectedValue"`
	ExpectedCost      float64       `json:"expectedCost"`
	ExpectedServiceMS int64         `json:"expectedServiceMs"`
	Priority          float64       `json:"priority"`
}

type EventRecord struct {
	Time       time.Time   `json:"time"`
	BundleID   string      `json:"bundleId"`
	BundleHash string      `json:"bundleHash"`
	From       BundleState `json:"from,omitempty"`
	To         BundleState `json:"to"`
	Reason     string      `json:"reason,omitempty"`
	Version    int64       `json:"version"`
	Sequence   uint64      `json:"sequence"`
	ClientID   string      `json:"clientId,omitempty"`
	RegionID   string      `json:"regionId,omitempty"`
	ShardID    string      `json:"shardId,omitempty"`
	LeaseID    string      `json:"leaseId,omitempty"`
	LeaseEpoch uint64      `json:"leaseEpoch,omitempty"`
	FenceToken uint64      `json:"fenceToken,omitempty"`
}

type CheckpointRecord struct {
	BatchID    string    `json:"batchId"`
	BundleID   string    `json:"bundleId"`
	ShardID    string    `json:"shardId"`
	ObjectKey  string    `json:"objectKey,omitempty"`
	Epoch      uint64    `json:"epoch"`
	Root       string    `json:"root"`
	EventCount int       `json:"eventCount"`
	LastOffset uint64    `json:"lastOffset"`
	RegionID   string    `json:"regionId"`
	SignedBy   string    `json:"signedBy"`
	Signature  string    `json:"signature"`
	Time       time.Time `json:"time"`
	Version    int64     `json:"version"`
}

type Decision struct {
	Action    string  `json:"action"`
	Reason    string  `json:"reason"`
	Score     float64 `json:"score"`
	ProfitEth float64 `json:"profitEth"`
	BlockHash string  `json:"blockHash,omitempty"`
}
