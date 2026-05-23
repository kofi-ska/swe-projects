package builder

import (
	"context"
	"log"
	"sync"
	"time"

	"mev-relay/internal/pb"
)

type Publisher interface {
	Publish(result *pb.BuildResult) error
}

type Service struct {
	pb.UnimplementedBuilderServiceServer
	mu        sync.Mutex
	pending   []*pb.BundleSubmission
	history   []*pb.BuildResult
	Publisher Publisher
}

func (s *Service) SubmitBundle(ctx context.Context, req *pb.BundleSubmission) (*pb.BuildResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("[Builder] Received bundle: %s, profit=%.6f ETH", req.BundleId, req.ProfitEth)

	s.pending = append(s.pending, req)

	block := BuildCandidateBlock(s.pending)
	ranked := RankBundles(block.Bundles)
	selected := ranked[0]

	log.Printf("[Builder] Selected bundle %s for inclusion", selected.BundleId)

	time.Sleep(300 * time.Millisecond)

	result := &pb.BuildResult{
		BlockHash:          GenerateBlockHash(selected.BundleId),
		Included:           true,
		InclusionReason:    "Highest profit",
		InclusionLatencyMs: 300,
	}

	s.history = append(s.history, result)
	s.pending = []*pb.BundleSubmission{}

	if s.Publisher != nil {
		if err := s.Publisher.Publish(result); err != nil {
			log.Printf("failed to publish build result: %v", err)
		}
	}

	return result, nil
}
