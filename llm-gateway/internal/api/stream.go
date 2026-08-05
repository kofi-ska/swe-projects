package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/kofi-ska/llm-gateway/internal/pipeline"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type requestOutcome string

const (
	outcomeSuccess       requestOutcome = "success"
	outcomeInvalid       requestOutcome = "invalid"
	outcomeCancelled     requestOutcome = "cancelled"
	outcomeOverloaded    requestOutcome = "overloaded"
	outcomeTimeout       requestOutcome = "timeout"
	outcomeInternalError requestOutcome = "internal_error"
)

type streamEvent struct {
	Token  string `json:"token,omitempty"`
	Done   bool   `json:"done,omitempty"`
	Tokens int    `json:"tokens,omitempty"`
	Error  string `json:"error,omitempty"`
}

func (h *Handler) stream(ctx context.Context, cancel context.CancelFunc, writer http.ResponseWriter, request *pipeline.Request, started time.Time, requestSpan trace.Span) requestOutcome {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		writeJSONError(writer, http.StatusInternalServerError, "streaming unsupported")
		return outcomeInternalError
	}
	writer.Header().Set("Content-Type", "application/x-ndjson")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	encoder := json.NewEncoder(writer)
	controller := http.NewResponseController(writer)
	_, span := otel.Tracer("llm-gateway/api").Start(ctx, "stream-response")
	defer span.End()

	firstToken := true
	var previousToken time.Time
	for {
		select {
		case <-ctx.Done():
			outcome := classifyContextError(ctx.Err())
			if outcome == outcomeTimeout {
				h.Metrics.TimedOut.Inc()
				requestSpan.SetStatus(codes.Error, "request timeout")
			} else {
				h.Metrics.Cancelled.Inc()
				span.AddEvent("client.cancelled")
			}
			return outcome
		case event := <-request.Events():
			if event.Err != nil {
				requestSpan.SetStatus(codes.Error, event.Err.Error())
				_ = writeStreamEvent(controller, encoder, flusher, h.WriteTimeout, streamEvent{Error: "inference failed"})
				return outcomeInternalError
			}
			if event.Token != "" {
				now := time.Now()
				if firstToken {
					h.Metrics.TTFT.Observe(now.Sub(started).Seconds())
					requestSpan.AddEvent("first.token")
					firstToken = false
				} else {
					h.Metrics.InterToken.Observe(now.Sub(previousToken).Seconds())
				}
				previousToken = now
				if err := writeStreamEvent(controller, encoder, flusher, h.WriteTimeout, streamEvent{Token: event.Token}); err != nil {
					h.Metrics.Cancelled.Inc()
					cancel()
					return outcomeCancelled
				}
				h.Metrics.Tokens.Inc()
			}
			if event.Done {
				if err := writeStreamEvent(controller, encoder, flusher, h.WriteTimeout, streamEvent{Done: true, Tokens: event.Tokens}); err != nil {
					h.Metrics.Cancelled.Inc()
					cancel()
					return outcomeCancelled
				}
				requestSpan.SetAttributes(attribute.Int("response.token_count", event.Tokens))
				return outcomeSuccess
			}
		}
	}
}

func writeStreamEvent(controller *http.ResponseController, encoder *json.Encoder, flusher http.Flusher, timeout time.Duration, event streamEvent) error {
	_ = controller.SetWriteDeadline(time.Now().Add(timeout))
	if err := encoder.Encode(event); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
