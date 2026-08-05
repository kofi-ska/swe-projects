package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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

type Handler struct {
	Pipeline     *pipeline.Pipeline
	Metrics      *observability.Metrics
	Timeout      time.Duration
	WriteTimeout time.Duration
	shuttingDown atomic.Bool
}

func (h *Handler) SetShuttingDown(value bool) { h.shuttingDown.Store(value) }

func (h *Handler) Generate(writer http.ResponseWriter, request *http.Request) {
	started := time.Now()
	outcome := outcomeInternalError
	h.Metrics.ActiveRequests.Inc()
	defer func() {
		h.Metrics.ActiveRequests.Dec()
		h.Metrics.Requests.WithLabelValues(request.Method, route, string(outcome)).Inc()
		h.Metrics.RequestDuration.WithLabelValues(route, string(outcome)).Observe(time.Since(started).Seconds())
	}()

	if request.Method != http.MethodPost {
		outcome = outcomeInvalid
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	input, err := decodeGenerateRequest(writer, request)
	if err != nil {
		outcome = outcomeInvalid
		writeJSONError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if h.shuttingDown.Load() {
		outcome = outcomeOverloaded
		writeUnavailable(writer, "service is shutting down")
		return
	}

	ctx, cancel := context.WithTimeout(request.Context(), h.Timeout)
	defer cancel()
	ctx, span := otel.Tracer("llm-gateway/api").Start(ctx, "POST "+route)
	defer span.End()
	span.SetAttributes(
		attribute.Int("request.max_tokens", input.MaxTokens),
		attribute.Int("request.prompt_length", utf8.RuneCountInString(input.Prompt)),
		attribute.String("request.priority", input.Priority),
	)

	work := pipeline.NewRequest(ctx, input.MaxTokens, input.Priority)
	_, enqueueSpan := otel.Tracer("llm-gateway/pipeline").Start(ctx, "enqueue-request")
	err = h.Pipeline.Enqueue(work)
	enqueueSpan.End()
	if err != nil {
		outcome = outcomeOverloaded
		h.Metrics.QueueRejections.Inc()
		span.SetStatus(codes.Error, err.Error())
		writeUnavailable(writer, "gateway overloaded")
		return
	}

	outcome = h.stream(ctx, cancel, writer, work, started, span)
}

func writeUnavailable(writer http.ResponseWriter, message string) {
	writer.Header().Set("Retry-After", "1")
	writeJSONError(writer, http.StatusServiceUnavailable, message)
}

func writeJSONError(writer http.ResponseWriter, status int, message string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]string{"error": message})
}

func classifyContextError(err error) requestOutcome {
	if errors.Is(err, context.DeadlineExceeded) {
		return outcomeTimeout
	}
	return outcomeCancelled
}
