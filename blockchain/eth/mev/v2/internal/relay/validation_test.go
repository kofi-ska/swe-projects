package relay

import (
	"testing"

	"mevrelayv2/internal/model"
)

func FuzzValidateBundleRequest(f *testing.F) {
	f.Add("2.0", "eth_sendBundle", []byte("0x1"), "0x1")
	f.Add("", "", []byte{}, "")
	f.Fuzz(func(t *testing.T, jsonrpc, method string, txsRaw []byte, blockNumber string) {
		txs := parseTxs(txsRaw)
		req := mustRequest(jsonrpc, method, txs, blockNumber)
		_ = validateBundleRequest(req)
	})
}

func mustRequest(jsonrpc, method string, txs []string, blockNumber string) model.JSONRPCRequest {
	return model.JSONRPCRequest{
		JSONRPC: jsonrpc,
		Method:  method,
		Params: []model.BundleRequest{{
			Txs:         txs,
			BlockNumber: blockNumber,
		}},
	}
}

func parseTxs(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, 4)
	cur := make([]byte, 0, len(raw))
	for _, b := range raw {
		if b == ',' || b == ';' || b == '\n' || b == '\t' || b == ' ' {
			if len(cur) > 0 {
				out = append(out, string(cur))
				cur = cur[:0]
			}
			continue
		}
		cur = append(cur, b)
	}
	if len(cur) > 0 {
		out = append(out, string(cur))
	}
	return out
}
