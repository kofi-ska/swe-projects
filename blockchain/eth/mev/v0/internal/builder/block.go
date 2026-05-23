package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"mev-relay/internal/pb"
)

// BlockCandidate represents a temporary block being constructed
type BlockCandidate struct {
	Bundles []*pb.BundleSubmission
}

// BuildCandidateBlock creates a new candidate block from queued bundles
func BuildCandidateBlock(bundles []*pb.BundleSubmission) *BlockCandidate {
	return &BlockCandidate{
		Bundles: bundles,
	}
}

// GenerateBlockHash simulates producing a deterministic hash for a built block
func GenerateBlockHash(bundleID string) string {
	sum := sha256.Sum256([]byte(bundleID))
	return "0x" + hex.EncodeToString(sum[:8]) // short hash for readability
}
