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

type Caller interface {
	CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error
}

func Dial(url string) (*rpc.Client, error) {
	return rpc.Dial(url)
}

func EndpointLabel(url string) string {
	sum := sha256.Sum256([]byte(url))
	return hex.EncodeToString(sum[:8])
}

func ChainID(ctx context.Context, c Caller) (*big.Int, error) {
	var raw string
	if err := c.CallContext(ctx, &raw, "eth_chainId"); err != nil {
		return nil, err
	}
	return parseBigIntString(raw)
}

func BlockNumber(ctx context.Context, c Caller) (*big.Int, error) {
	var raw string
	if err := c.CallContext(ctx, &raw, "eth_blockNumber"); err != nil {
		return nil, err
	}
	return parseBigIntString(raw)
}

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

func ClientVersion(ctx context.Context, c Caller) (string, error) {
	var raw string
	if err := c.CallContext(ctx, &raw, "web3_clientVersion"); err != nil {
		return "", err
	}
	return raw, nil
}

type BlockHeader struct {
	Number        string `json:"number"`
	GasLimit      string `json:"gasLimit"`
	BaseFeePerGas string `json:"baseFeePerGas"`
}

func BlockHeaderByNumber(ctx context.Context, c Caller, number string) (BlockHeader, error) {
	var h BlockHeader
	if err := c.CallContext(ctx, &h, "eth_getBlockByNumber", number, false); err != nil {
		return BlockHeader{}, err
	}
	return h, nil
}

func TxPoolContent(ctx context.Context, c Caller) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.CallContext(ctx, &raw, "txpool_content"); err != nil {
		return nil, err
	}
	return raw, nil
}

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
