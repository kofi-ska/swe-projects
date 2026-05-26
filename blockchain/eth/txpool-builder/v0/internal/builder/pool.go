package builder

import (
	"context"
	"encoding/json"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/rpc"
)

// TxMeta represents simplified metadata for a transaction extracted from the txpool.
type TxMeta struct {
	Hash     string   `json:"hash"`
	From     string   `json:"from"`
	GasPrice *big.Int `json:"gas_price"`
	Nonce    uint64   `json:"nonce"`
}

// FetchPendingTxs retrieves all pending transactions from the Geth txpool.
// It uses the txpool_content RPC call; returning a slice of TxMeta objects.
func FetchPendingTxs(ctx context.Context, rpcClient *rpc.Client) []TxMeta {
	var result map[string]map[string]map[string]map[string]interface{}
	err := rpcClient.CallContext(ctx, &result, "txpool_content")
	if err != nil {
		log.Fatalf("failed to call txpool_content: %v", err)
	}

	txs := []TxMeta{}
	for _, senders := range result["pending"] {
		for _, txData := range senders {
			for _, raw := range txData {
				txMap := raw.(map[string]interface{})
				gasStr := txMap["gasPrice"].(string)
				gas, _ := new(big.Int).SetString(gasStr[2:], 16)

				txs = append(txs, TxMeta{
					Hash:     txMap["hash"].(string),
					From:     txMap["from"].(string),
					GasPrice: gas,
					Nonce:    uint64(txMap["nonce"].(float64)),
				})
			}
		}
	}

	log.Printf("fetched %d pending transactions from pool", len(txs))

	if len(txs) < 10 {
		b, _ := json.MarshalIndent(txs, "", "  ")
		log.Println(string(b))
	}

	return txs
}
