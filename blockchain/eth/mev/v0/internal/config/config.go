package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	// Common
	DatabaseURL string

	// Service ports
	RelayPort     string
	SimulatorPort string
	BuilderPort   string

	// RPC / External connections
	AnvilRPC string
	GethRPC  string

	// Optional flags or settings
	Env string
}

func Load() *Config {
	// Load .env file if present
	_ = godotenv.Load()

	cfg := &Config{
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/mevrelay?sslmode=disable"),
		RelayPort:     getEnv("RELAY_PORT", "8080"),
		SimulatorPort: getEnv("SIMULATOR_PORT", "50051"),
		BuilderPort:   getEnv("BUILDER_PORT", "50052"),
		AnvilRPC:      getEnv("ANVIL_RPC", "http://anvil:8545"),
		GethRPC:       getEnv("GETH_RPC", "http://geth:8545"),
		Env:           getEnv("ENV", "development"),
	}

	log.Println("[Config] Loaded configuration for", cfg.Env)
	return cfg
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
