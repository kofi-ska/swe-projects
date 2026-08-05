package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kofi-ska/llm-gateway/internal/api"
	"github.com/kofi-ska/llm-gateway/internal/config"
	"github.com/kofi-ska/llm-gateway/internal/observability"
	"github.com/kofi-ska/llm-gateway/internal/pipeline"
	"github.com/kofi-ska/llm-gateway/internal/worker"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	tp, err := observability.InitTracing(ctx, cfg.OTLPEndpoint, cfg.OTELSampleRate)
	if err != nil {
		slog.Error("initialize tracing", "error", err)
		os.Exit(1)
	}
	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	metrics := observability.NewMetrics(reg, cfg.QueueCapacity)
	p := pipeline.New(cfg.QueueCapacity, cfg.MaxBatchSize, cfg.WorkerCount, cfg.MaxBatchWait, worker.Worker{BaseDelay: cfg.BaseInferenceDelay, TokenDelay: cfg.PerTokenDelay, FailureRate: cfg.FailureRate}, metrics)
	p.Start()
	defer p.Stop()
	h := &api.Handler{Pipeline: p, Metrics: metrics, Timeout: cfg.RequestTimeout}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/generate", h.Generate)
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) { writeHealth(w, http.StatusOK, "live") })
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		ready := !isDone(ctx) && p.Running() && p.Workers() > 0 && float64(p.QueueDepth())/float64(p.QueueCapacity()) < cfg.ReadinessSaturation
		if !ready {
			writeHealth(w, http.StatusServiceUnavailable, "not ready")
			return
		}
		writeHealth(w, http.StatusOK, "ready")
	})
	server := &http.Server{Addr: ":" + cfg.HTTPPort, Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	admin := &http.Server{Addr: cfg.PprofAddress, Handler: http.DefaultServeMux, ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 2)
	go func() { slog.Info("gateway listening", "address", server.Addr); errCh <- server.ListenAndServe() }()
	go func() { slog.Info("pprof listening", "address", admin.Addr); errCh <- admin.ListenAndServe() }()
	select {
	case <-ctx.Done():
		slog.Info("shutdown requested")
	case err = <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server stopped", "error", err)
		}
	}
	h.SetShuttingDown(true)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	p.Stop()
	_ = admin.Shutdown(shutdownCtx)
	if err := tp.Shutdown(shutdownCtx); err != nil {
		slog.Warn("flush traces", "error", err)
	}
}

func writeHealth(w http.ResponseWriter, status int, state string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": state})
}
func isDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
