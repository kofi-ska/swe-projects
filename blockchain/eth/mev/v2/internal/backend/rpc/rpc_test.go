package rpc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"mevrelayv2/internal/model"
)

func TestRPCAdapter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var result any
		switch req.Method {
		case "eth_chainId":
			result = "0x1"
		case "eth_blockNumber":
			result = "0x10"
		default:
			http.Error(w, "unexpected method", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: func() json.RawMessage {
				raw, _ := json.Marshal(result)
				return raw
			}(),
		})
	}))
	defer srv.Close()

	a, err := New("sepolia", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	res, err := a.Simulate(context.Background(), model.BundleRecord{
		Request: model.BundleRequest{Txs: []string{"0x1", "0x2"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Score == 0 {
		t.Fatalf("expected score, got %#v", res)
	}
}
