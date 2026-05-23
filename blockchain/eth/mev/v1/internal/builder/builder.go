package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"mevrelayv1/internal/model"
)

// Decide converts a simulation result into a relay decision.
func Decide(rec model.BundleRecord, sim model.SimulationResult) model.Decision {
	score := sim.ProfitEth
	if sim.Success && score > 0 {
		return model.Decision{
			Action:    "forward",
			Reason:    "profitable",
			Score:     score,
			ProfitEth: sim.ProfitEth,
			BlockHash: blockHash(rec.BundleHash, rec.Version),
		}
	}

	return model.Decision{
		Action:    "reject",
		Reason:    sim.Reason,
		Score:     score,
		ProfitEth: sim.ProfitEth,
	}
}

func blockHash(bundleHash string, version int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", bundleHash, version)))
	return "0x" + hex.EncodeToString(sum[:8])
}
