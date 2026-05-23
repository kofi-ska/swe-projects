package relay

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"mev-relay/internal/config"
	"mev-relay/internal/pb"
)

// dispatchToSimulator sends the bundle to the Simulator gRPC service
func dispatchToSimulator(cfg *config.Config, bundle *pb.BundleRequest) (*pb.BundleResponse, error) {
	conn, err := grpc.Dial(cfg.SimulatorPort, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := pb.NewSimulationServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.SimulateBundle(ctx, bundle)
	if err != nil {
		return nil, err
	}

	log.Printf("[Relay] Simulation result for bundle %s: profit=%.6f ETH success=%v",
		resp.BundleId, resp.ProfitEth, resp.Success)

	return resp, nil
}
