package relay

import (
	"context"
	"errors"
	"net/http"

	"mevrelayv2/internal/scheduler"
	coordstate "mevrelayv2/internal/state"
)

var (
	ErrInvalidJSONRPC       = errors.New("invalid jsonrpc version")
	ErrInvalidMethod        = errors.New("invalid method")
	ErrMissingParams        = errors.New("missing params")
	ErrMissingTxs           = errors.New("missing txs")
	ErrMissingAuthorization = errors.New("missing authorization")
	ErrInvalidAuthorization = errors.New("invalid authorization")
	ErrDuplicateBundle      = errors.New("duplicate bundle")
	ErrQueueClosed          = errors.New("queue closed")
	ErrQueueDisabled        = errors.New("queue disabled")
	ErrQueueOverflow        = errors.New("queue overflow")
	ErrStaleWork            = errors.New("stale work")
	ErrClientInflightLimit  = errors.New("client inflight limit")
	ErrStateCapacity        = errors.New("state capacity")
	ErrInsufficientDeadline = errors.New("insufficient slack")
	ErrStaleDeadline        = errors.New("stale deadline")
	ErrNegativeNetValue     = errors.New("negative net value")
)

// statusForError maps internal errors to stable HTTP status codes.
func statusForError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrDuplicateBundle):
		return http.StatusConflict
	case errors.Is(err, coordstate.ErrDuplicateBundle):
		return http.StatusConflict
	case errors.Is(err, ErrMissingAuthorization), errors.Is(err, ErrInvalidAuthorization):
		return http.StatusUnauthorized
	case errors.Is(err, ErrInvalidJSONRPC), errors.Is(err, ErrInvalidMethod), errors.Is(err, ErrMissingParams), errors.Is(err, ErrMissingTxs):
		return http.StatusBadRequest
	case errors.Is(err, ErrStaleWork), errors.Is(err, ErrStaleDeadline), errors.Is(err, ErrInsufficientDeadline), errors.Is(err, ErrNegativeNetValue):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrQueueOverflow), errors.Is(err, ErrQueueDisabled), errors.Is(err, ErrQueueClosed), errors.Is(err, scheduler.ErrQueueOverflow), errors.Is(err, scheduler.ErrQueueDisabled), errors.Is(err, scheduler.ErrQueueClosed):
		return http.StatusServiceUnavailable
	case errors.Is(err, ErrClientInflightLimit), errors.Is(err, coordstate.ErrClientInflight), errors.Is(err, coordstate.ErrStateCapacity), errors.Is(err, coordstate.ErrStateClosed):
		return http.StatusServiceUnavailable
	case errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout
	default:
		return http.StatusBadRequest
	}
}
