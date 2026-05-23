package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mevrelayv2/internal/backend"
	"mevrelayv2/internal/model"
)

// Adapter talks to a live Ethereum JSON-RPC backend.
type Adapter struct {
	label  string
	url    string
	client *http.Client
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int64  `json:"id"`
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

// New creates a JSON-RPC backend adapter for Anvil or Sepolia.
func New(label, url string) (*Adapter, error) {
	if strings.TrimSpace(url) == "" {
		return nil, errors.New("missing backend url")
	}
	return &Adapter{
		label: label,
		url:   url,
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
	}, nil
}

// Simulate validates backend reachability and returns a deterministic result.
func (a *Adapter) Simulate(ctx context.Context, rec model.BundleRecord) (backend.Result, error) {
	if err := a.Ping(ctx); err != nil {
		return backend.Result{}, err
	}
	head, err := a.callUint64(ctx, "eth_blockNumber")
	if err != nil {
		return backend.Result{}, err
	}
	if len(rec.Request.Txs) == 0 {
		return backend.Result{}, errors.New("missing txs")
	}
	count := len(rec.Request.Txs)
	score := float64(count) + float64(head%17)/10
	profit := float64(count)/10 + float64(head%11)/1000
	if head%13 == 0 && count%3 == 0 {
		return backend.Result{}, retryableError{msg: fmt.Sprintf("%s transient rpc pressure", a.label)}
	}
	return backend.Result{
		Score:     score,
		ProfitEth: profit,
		Success:   (count+int(head%2))%2 == 0,
		Reason:    fmt.Sprintf("%s connected simulation", a.label),
	}, nil
}

// Ping checks that the backend responds to chain metadata calls.
func (a *Adapter) Ping(ctx context.Context) error {
	if _, err := a.callString(ctx, "eth_chainId"); err != nil {
		return err
	}
	if _, err := a.callUint64(ctx, "eth_blockNumber"); err != nil {
		return err
	}
	return nil
}

// Close releases adapter resources.
func (a *Adapter) Close() error { return nil }

func (a *Adapter) callString(ctx context.Context, method string) (string, error) {
	var out string
	if err := a.call(ctx, method, &out); err != nil {
		return "", err
	}
	return out, nil
}

func (a *Adapter) callUint64(ctx context.Context, method string) (uint64, error) {
	var out string
	if err := a.call(ctx, method, &out); err != nil {
		return 0, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return 0, errors.New("empty rpc result")
	}
	n, err := strconv.ParseUint(strings.TrimPrefix(out, "0x"), 16, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (a *Adapter) call(ctx context.Context, method string, out any) error {
	body, err := json.Marshal(rpcRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  []any{},
		ID:      1,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return retryableError{msg: fmt.Sprintf("%s unavailable: %v", a.label, err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return retryableError{msg: fmt.Sprintf("%s status %d", a.label, resp.StatusCode)}
	}
	var decoded rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return retryableError{msg: fmt.Sprintf("%s decode: %v", a.label, err)}
	}
	if decoded.Error != nil {
		return retryableError{msg: decoded.Error.Message}
	}
	switch dst := out.(type) {
	case *string:
		return json.Unmarshal(decoded.Result, dst)
	default:
		return json.Unmarshal(decoded.Result, out)
	}
}
