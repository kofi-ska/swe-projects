package builder

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

// BuildBlock assembles the top N ranked transactions into a pseudo block.
// The block is serialized as JSON and written to disk.
func BuildBlock(txs []TxMeta, limit int) {
	if len(txs) > limit {
		txs = txs[:limit]
	}

	block := map[string]interface{}{
		"timestamp": time.Now().UTC(),
		"tx_count":  len(txs),
		"txs":       txs,
	}

	data, err := json.MarshalIndent(block, "", "  ")
	if err != nil {
		log.Fatalf("failed to encode pseudo block: %v", err)
	}

	if err := os.WriteFile("pseudo_block.json", data, 0644); err != nil {
		log.Fatalf("failed to write file: %v", err)
	}
}
