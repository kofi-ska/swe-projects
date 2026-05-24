package relay

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"mevrelayv2/internal/scheduler"
	coordstate "mevrelayv2/internal/state"
)

func TestStatusForError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "duplicate bundle", err: coordstate.ErrDuplicateBundle, want: http.StatusConflict},
		{name: "invalid jsonrpc", err: ErrInvalidJSONRPC, want: http.StatusBadRequest},
		{name: "queue overflow", err: scheduler.ErrQueueOverflow, want: http.StatusServiceUnavailable},
		{name: "state capacity", err: coordstate.ErrStateCapacity, want: http.StatusServiceUnavailable},
		{name: "deadline", err: context.DeadlineExceeded, want: http.StatusGatewayTimeout},
		{name: "stale work", err: ErrStaleWork, want: http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusForError(tt.err); got != tt.want {
				t.Fatalf("statusForError() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestStatusForErrorFallsBackToBadRequest(t *testing.T) {
	if got := statusForError(errors.New("unknown")); got != http.StatusBadRequest {
		t.Fatalf("statusForError() = %d, want %d", got, http.StatusBadRequest)
	}
}
