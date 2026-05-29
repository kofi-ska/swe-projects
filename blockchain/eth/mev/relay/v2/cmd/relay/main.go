package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"mevrelayv2/internal/backend"
	"mevrelayv2/internal/backend/local"
	"mevrelayv2/internal/backend/rpc"
	"mevrelayv2/internal/broker"
	"mevrelayv2/internal/broker/memory"
	"mevrelayv2/internal/broker/nats"
	"mevrelayv2/internal/config"
	"mevrelayv2/internal/eventlog"
	"mevrelayv2/internal/relay"
	coordstate "mevrelayv2/internal/state"
	stateMemory "mevrelayv2/internal/state/memory"
)

func main() {
	cfg := config.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wal, err := eventlog.Open(cfg.DataDir+"/wal.jsonl", cfg.AuditFlushEvery, cfg.WALMaxEntries)
	if err != nil {
		log.Fatal(err)
	}
	defer wal.Close()

	var be backend.Adapter
	switch strings.ToLower(cfg.BackendKind) {
	case string(backend.KindLocal):
		be = local.New()
	case string(backend.KindSepolia), string(backend.KindAnvil):
		be, err = rpc.New(strings.ToLower(cfg.BackendKind), cfg.BackendURL)
		if err != nil {
			log.Fatal(err)
		}
	default:
		be, err = rpc.New(string(backend.KindAnvil), cfg.BackendURL)
		if err != nil {
			log.Fatal(err)
		}
	}
	defer be.Close()

	var st coordstate.Store
	switch strings.ToLower(cfg.StateKind) {
	case "valkey":
		st, err = coordstate.NewValkey(cfg.ValkeyURL, cfg.StateRetention, cfg.HistoryLimit)
		if err != nil {
			log.Fatal(err)
		}
	case "memory":
		fallthrough
	default:
		st = stateMemory.New()
	}
	defer st.Close()

	var br broker.Broker
	switch strings.ToLower(cfg.BrokerKind) {
	case "nats":
		br, err = nats.New(cfg.NATSURL)
		if err != nil {
			log.Fatal(err)
		}
	case "memory":
		fallthrough
	default:
		br = memory.New(cfg.BrokerBuffer)
	}
	defer br.Close()

	svc := relay.New(cfg, be, br, st, wal)
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
		log.Printf("v2 relay listening on %s", cfg.HTTPAddr)
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
