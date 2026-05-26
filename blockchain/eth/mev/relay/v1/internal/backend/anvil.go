package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"mevrelayv1/internal/model"
)

type Anvil struct {
	RPCURL string
	Client *http.Client
}

type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int64         `json:"id"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type retryableError struct{ msg string }

func (e retryableError) Error() string   { return e.msg }
func (e retryableError) Retryable() bool { return true }

// NewAnvil constructs an Anvil-backed simulator client.
func NewAnvil(url string) *Anvil {
	return &Anvil{
		RPCURL: url,
		Client: &http.Client{Timeout: 2 * time.Second},
	}
}

func (a *Anvil) Simulate(ctx context.Context, rec model.BundleRecord) (model.SimulationResult, error) {
	if a.RPCURL == "" {
		return model.SimulationResult{}, retryableError{msg: "missing rpc url"}
	}

	reqBody, _ := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		Method:  "eth_chainId",
		Params:  []interface{}{},
		ID:      1,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.RPCURL, bytes.NewReader(reqBody))
	if err != nil {
		return model.SimulationResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.Client.Do(req)
	if err != nil {
		return model.SimulationResult{}, retryableError{msg: fmt.Sprintf("backend unavailable: %v", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return model.SimulationResult{}, retryableError{msg: fmt.Sprintf("backend status %d", resp.StatusCode)}
	}

	var decoded rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return model.SimulationResult{}, retryableError{msg: fmt.Sprintf("backend decode: %v", err)}
	}
	if decoded.Error != nil {
		return model.SimulationResult{}, retryableError{msg: decoded.Error.Message}
	}

	// Deterministic local scoring path for v1.
	txCount := len(rec.Request.Txs)
	profit := float64(txCount) * 0.001
	latency := int64(10 + txCount*3)
	success := txCount > 0
	reason := "ok"
	if !success {
		reason = "empty bundle"
	}

	return model.SimulationResult{
		ProfitEth: profit,
		LatencyMS: latency,
		Success:   success,
		Reason:    reason,
	}, nil
}

func (a *Anvil) Ping(ctx context.Context) error {
	if a.RPCURL == "" {
		return retryableError{msg: "missing rpc url"}
	}

	reqBody, _ := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		Method:  "eth_chainId",
		Params:  []interface{}{},
		ID:      1,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.RPCURL, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.Client.Do(req)
	if err != nil {
		return retryableError{msg: fmt.Sprintf("backend unavailable: %v", err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return retryableError{msg: fmt.Sprintf("backend status %d", resp.StatusCode)}
	}
	return nil
}
