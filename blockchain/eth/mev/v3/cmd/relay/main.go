package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mevrelayv3/internal/app"
	"mevrelayv3/internal/config"
	"mevrelayv3/internal/relay"
	"mevrelayv3/internal/telemetry"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdownTrace, err := telemetry.InitTracing(ctx, telemetry.TraceConfig{
		ServiceName: cfg.OTELServiceName,
		Endpoint:    cfg.OTELExporterEndpoint,
		Insecure:    cfg.OTELExporterInsecure,
		SampleRatio: cfg.OTELSampleRatio,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = shutdownTrace(context.Background())
	}()

	rt, err := app.Build(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer rt.Close()

	svc := rt.Service
	if err := svc.Bootstrap(ctx); err != nil {
		log.Fatal(err)
	}
	svc.Start(ctx)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           relay.Handler{Svc: svc},
		ReadHeaderTimeout: 2 * time.Second,
	}

	go func() {
		log.Printf("v3 relay listening on %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	svc.Drain()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	_ = server.Shutdown(shutdownCtx)
	svc.Stop()
}
