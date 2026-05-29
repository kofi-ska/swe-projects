package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mevrelayv1/internal/backend"
	"mevrelayv1/internal/config"
	"mevrelayv1/internal/metrics"
	"mevrelayv1/internal/relay"
	"mevrelayv1/internal/store"
)

func main() {
	cfg := config.Load()

	st, err := store.NewFileStore(cfg.DataDir)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	m := &metrics.Metrics{}
	svc := relay.New(cfg, st, backend.NewAnvil(cfg.RPCURL), m)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           relay.Handler{Svc: svc},
		ReadHeaderTimeout: 2 * time.Second,
	}

	go func() {
		log.Printf("relay listening on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	_ = server.Shutdown(shutdownCtx)
	svc.Stop()
}
