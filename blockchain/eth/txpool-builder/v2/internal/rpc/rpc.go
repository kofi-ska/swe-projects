package rpcx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/rpc"
)

// Caller stays tiny so the service can be tested without a full client.
type Caller interface {
	CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error
}

// Dial is isolated here so transport setup stays out of the service package.
func Dial(url string) (*rpc.Client, error) {
	return rpc.Dial(url)
}

// EndpointLabel hides the raw URL while still letting runs be correlated.
func EndpointLabel(url string) string {
	sum := sha256.Sum256([]byte(url))
	return hex.EncodeToString(sum[:8])
}

// ChainID is a startup gate so the service never builds for the wrong chain.
func ChainID(ctx context.Context, c Caller) (*big.Int, error) {
	var raw string
	if err := c.CallContext(ctx, &raw, "eth_chainId"); err != nil {
		return nil, err
	}
	return parseBigIntString(raw)
}

// BlockNumber anchors the snapshot to a concrete head for replay safety.
func BlockNumber(ctx context.Context, c Caller) (*big.Int, error) {
	var raw string
	if err := c.CallContext(ctx, &raw, "eth_blockNumber"); err != nil {
		return nil, err
	}
	return parseBigIntString(raw)
}

// Syncing blocks capture while the upstream node is not ready to serve truth.
func Syncing(ctx context.Context, c Caller) (bool, error) {
	var raw json.RawMessage
	if err := c.CallContext(ctx, &raw, "eth_syncing"); err != nil {
		return false, err
	}
	if len(raw) == 0 || string(raw) == "false" {
		return false, nil
	}
	if string(raw) == "true" {
		return true, nil
	}
	return true, nil
}

// ClientVersion is stored so operator reports can explain source behavior.
func ClientVersion(ctx context.Context, c Caller) (string, error) {
	var raw string
	if err := c.CallContext(ctx, &raw, "web3_clientVersion"); err != nil {
		return "", err
	}
	return raw, nil
}

// BlockHeader carries only the header fields needed to bound selection.
type BlockHeader struct {
	Number        string `json:"number"`
	GasLimit      string `json:"gasLimit"`
	BaseFeePerGas string `json:"baseFeePerGas"`
}

// BlockHeaderByNumber keeps header fetches separate from pool capture.
func BlockHeaderByNumber(ctx context.Context, c Caller, number string) (BlockHeader, error) {
	var h BlockHeader
	if err := c.CallContext(ctx, &h, "eth_getBlockByNumber", number, false); err != nil {
		return BlockHeader{}, err
	}
	return h, nil
}

// TxPoolContent is the raw source so normalization can stay deterministic.
func TxPoolContent(ctx context.Context, c Caller) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.CallContext(ctx, &raw, "txpool_content"); err != nil {
		return nil, err
	}
	return raw, nil
}

// parseBigIntString keeps RPC number parsing strict so bad headers fail early.
func parseBigIntString(s string) (*big.Int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty hex number")
	}
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		n, ok := new(big.Int).SetString(s[2:], 16)
		if !ok {
			return nil, fmt.Errorf("invalid hex number: %s", s)
		}
		return n, nil
	}
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, fmt.Errorf("invalid number: %s", s)
	}
	return n, nil
}
