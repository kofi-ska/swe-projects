package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"mev-relay/internal/config"
	"mev-relay/internal/pb"
	"mev-relay/internal/simulator"
)

func main() {
	cfg := config.Load()

	lis, err := net.Listen("tcp", ":"+cfg.SimulatorPort)
	if err != nil {
		log.Fatal("Failed to listen:", err)
	}

	server := grpc.NewServer()

	simulatorService := simulator.NewService(cfg)
	pb.RegisterSimulationServiceServer(server, simulatorService)

	log.Println("Simulation service running on port", cfg.SimulatorPort)
	if err := server.Serve(lis); err != nil {
		log.Fatal("Simulator service failed:", err)
	}
}
