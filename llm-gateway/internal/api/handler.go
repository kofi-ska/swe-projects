package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/kofi-ska/llm-gateway/internal/observability"
	"github.com/kofi-ska/llm-gateway/internal/pipeline"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const route = "/v1/generate"

type GenerateRequest struct {
	Prompt    string `json:"prompt"`
	MaxTokens int    `json:"max_tokens"`
	Priority  string `json:"priority"`
}
type streamEvent struct {
	Token  string `json:"token,omitempty"`
	Done   bool   `json:"done,omitempty"`
	Tokens int    `json:"tokens,omitempty"`
	Error  string `json:"error,omitempty"`
}

type Handler struct {
	Pipeline     *pipeline.Pipeline
	Metrics      *observability.Metrics
	Timeout      time.Duration
	shuttingDown atomic.Bool
}

func (h *Handler) SetShuttingDown(v bool) { h.shuttingDown.Store(v) }

func (h *Handler) Generate(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	outcome := "internal_error"
	h.Metrics.ActiveRequests.Inc()
	defer func() {
		h.Metrics.ActiveRequests.Dec()
		h.Metrics.Requests.WithLabelValues(r.Method, route, outcome).Inc()
		h.Metrics.RequestDuration.WithLabelValues(route, outcome).Observe(time.Since(started).Seconds())
	}()
	if r.Method != http.MethodPost {
		outcome = "invalid"
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in GenerateRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		outcome = "invalid"
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request")
		return
	}
	if msg := validate(in); msg != "" {
		outcome = "invalid"
		writeJSONError(w, http.StatusBadRequest, msg)
		return
	}
	if h.shuttingDown.Load() {
		outcome = "overloaded"
		w.Header().Set("Retry-After", "1")
		writeJSONError(w, http.StatusServiceUnavailable, "service is shutting down")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.Timeout)
	defer cancel()
	ctx, span := otel.Tracer("llm-gateway/api").Start(ctx, "POST "+route)
	defer span.End()
	span.SetAttributes(attribute.Int("request.max_tokens", in.MaxTokens), attribute.Int("request.prompt_length", utf8.RuneCountInString(in.Prompt)), attribute.String("request.priority", in.Priority))
	events := make(chan pipeline.Event, 1)
	req := &pipeline.Request{Context: ctx, MaxTokens: in.MaxTokens, Priority: in.Priority, Enqueued: time.Now(), Events: events}
	_, enqueueSpan := otel.Tracer("llm-gateway/pipeline").Start(ctx, "enqueue-request")
	err := h.Pipeline.Enqueue(req)
	enqueueSpan.End()
	if err != nil {
		outcome = "overloaded"
		h.Metrics.QueueRejections.Inc()
		span.SetStatus(codes.Error, "queue full")
		w.Header().Set("Retry-After", "1")
		writeJSONError(w, http.StatusServiceUnavailable, "gateway overloaded")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		outcome = "internal_error"
		writeJSONError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	enc := json.NewEncoder(w)
	_, streamSpan := otel.Tracer("llm-gateway/api").Start(ctx, "stream-response")
	defer streamSpan.End()
	first := true
	var last time.Time
	for {
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				outcome = "timeout"
				h.Metrics.TimedOut.Inc()
				span.SetStatus(codes.Error, "request timeout")
			} else {
				outcome = "cancelled"
				h.Metrics.Cancelled.Inc()
				streamSpan.AddEvent("client.cancelled")
			}
			return
		case e := <-events:
			if e.Err != nil {
				outcome = "internal_error"
				span.SetStatus(codes.Error, e.Err.Error())
				_ = enc.Encode(streamEvent{Error: "inference failed"})
				flusher.Flush()
				return
			}
			if e.Token != "" {
				now := time.Now()
				if first {
					h.Metrics.TTFT.Observe(now.Sub(started).Seconds())
					span.AddEvent("first.token")
					first = false
				} else {
					h.Metrics.InterToken.Observe(now.Sub(last).Seconds())
				}
				last = now
				h.Metrics.Tokens.Inc()
				_ = enc.Encode(streamEvent{Token: e.Token})
				flusher.Flush()
			}
			if e.Done {
				_ = enc.Encode(streamEvent{Done: true, Tokens: e.Tokens})
				flusher.Flush()
				outcome = "success"
				span.SetAttributes(attribute.Int("response.token_count", e.Tokens))
				return
			}
		}
	}
}

func validate(r GenerateRequest) string {
	if strings.TrimSpace(r.Prompt) == "" {
		return "prompt must not be empty"
	}
	if utf8.RuneCountInString(r.Prompt) > 8000 {
		return "prompt must not exceed 8000 characters"
	}
	if r.MaxTokens < 1 || r.MaxTokens > 500 {
		return "max_tokens must be between 1 and 500"
	}
	if r.Priority != "low" && r.Priority != "normal" && r.Priority != "high" {
		return "priority must be low, normal, or high"
	}
	return ""
}
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
