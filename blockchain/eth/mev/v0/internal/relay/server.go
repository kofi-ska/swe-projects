package relay

import (
	"log"

	"github.com/gin-gonic/gin"
	"mev-relay/internal/config"
)

// StartServer launches the JSON-RPC relay service that receives bundles from searchers.
func StartServer(cfg *config.Config) error {
	router := gin.Default()

	router.POST("/relay/v1/bundle", func(c *gin.Context) {
		handleBundleRequest(c, cfg)
	})

	addr := ":" + cfg.RelayPort
	log.Println("[Relay] Listening on", addr)

	return router.Run(addr)
}
