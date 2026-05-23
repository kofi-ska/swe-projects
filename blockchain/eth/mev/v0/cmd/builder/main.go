package main

import (
	"context"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"github.com/jackc/pgx/v5/pgxpool"

	"mev-relay/internal/builder"
	"mev-relay/internal/config"
	"mev-relay/internal/pb"
)

func main() {
	cfg := config.Load()

	dbPool, err := connectDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Database connection failed:", err)
	}
	defer dbPool.Close()

	lis, err := net.Listen("tcp", ":"+cfg.BuilderPort)
	if err != nil {
		log.Fatal("Failed to listen:", err)
	}

	server := grpc.NewServer()

	builderService := &builder.Service{
		Publisher: builder.NewDBPublisher(dbPool),
	}

	pb.RegisterBuilderServiceServer(server, builderService)

	log.Println("Builder service running on port", cfg.BuilderPort)
	if err := server.Serve(lis); err != nil {
		log.Fatal("Builder service failed:", err)
	}
}

func connectDB(databaseURL string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}

	log.Println("Connected to TimescaleDB")
	return pool, nil
}
