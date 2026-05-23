package relay

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"mev-relay/internal/config"
	"mev-relay/internal/pb"
)

// BundleRPCRequest defines the JSON-RPC schema for eth_sendBundle.
type BundleRPCRequest struct {
	JSONRPC string            `json:"jsonrpc"`
	ID      int               `json:"id"`
	Method  string            `json:"method"`
	Params  []BundleRPCParams `json:"params"`
}

type BundleRPCParams struct {
	Txs          []string `json:"txs"`
	BlockNumber  string   `json:"blockNumber"`
	MinTimestamp int64    `json:"minTimestamp"`
	MaxTimestamp int64    `json:"maxTimestamp"`
	Replacement  *string  `json:"replacementUuid,omitempty"`
}

// BundleRPCResponse defines the JSON-RPC response payload.
type BundleRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result"`
	Error   interface{} `json:"error,omitempty"`
}

func handleBundleRequest(c *gin.Context, cfg *config.Config) {
	var req BundleRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	if len(req.Params) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing params"})
		return
	}

	params := req.Params[0]
	log.Printf("[Relay] Received bundle submission with %d txs targeting block %s",
		len(params.Txs), params.BlockNumber)

	result, err := dispatchToSimulator(cfg, &pb.BundleRequest{
		BundleId:    generateBundleID(),
		Txs:         params.Txs,
		TargetBlock: params.BlockNumber,
	})
	if err != nil {
		log.Println("[Relay] Simulation error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := BundleRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: gin.H{
			"bundleHash":   result.BundleId,
			"profit_eth":   result.ProfitEth,
			"latency_ms":   result.LatencyMs,
			"simulated_at": time.Now().UTC(),
		},
	}

	c.JSON(http.StatusOK, resp)
}

func generateBundleID() string {
	return time.Now().Format("20060102T150405.000000000")
}
