package main

import (
	"log"
	"mev-relay/internal/relay"
	"mev-relay/internal/config"
)

func main() {
	cfg := config.Load()

	log.Println("Starting MEV Relay on port", cfg.RelayPort)
	if err := relay.StartServer(cfg); err != nil {
		log.Fatal("Relay server error:", err)
	}
}
