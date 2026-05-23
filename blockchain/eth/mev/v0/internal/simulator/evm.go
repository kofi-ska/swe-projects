package simulator

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"mev-relay/internal/config"
)

// RunSimulation executes bundle transactions in a forked Anvil or Geth instance.
// In this mock version, we simulate success and profitability.
func RunSimulation(cfg *config.Config, txs []string) (bool, float64, error) {
	// Example: call eth_call or debug_traceCall for first tx to ensure connectivity
	_, err := callRPC(cfg.AnvilRPC, "eth_chainId", []interface{}{})
	if err != nil {
		return false, 0, fmt.Errorf("node unavailable: %w", err)
	}

	// Simulate computation time
	time.Sleep(time.Duration(500+rand.Intn(400)) * time.Millisecond)

	// Randomized profit to mimic variable outcomes
	profit := 0.001 + rand.Float64()*0.005
	success := rand.Intn(100) > 10 // 90% chance of success

	// For demonstration, serialize results as trace output
	traceData, _ := json.Marshal(map[string]interface{}{
		"tx_count": len(txs),
		"profit":   profit,
		"success":  success,
	})
	_ = traceData

	return success, profit, nil
}
