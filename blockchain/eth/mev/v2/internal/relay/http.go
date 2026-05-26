package relay

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"mevrelayv2/internal/model"
)

// Handler exposes the v2 relay over HTTP.
type Handler struct {
	Svc *Service
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/healthz":
		h.writeHealth(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/readyz":
		h.writeReady(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/metrics":
		h.writeMetrics(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/relay/v2/bundle":
		h.submitBundle(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/relay/v2/bundle"):
		h.getBundle(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h Handler) writeHealth(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, h.Svc.AssessHealth(r.Context()))
}

func (h Handler) writeReady(w http.ResponseWriter, r *http.Request) {
	report := h.Svc.AssessHealth(r.Context())
	if !report.Ready {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	h.writeJSON(w, report)
}

func (h Handler) writeMetrics(w http.ResponseWriter, r *http.Request) {
	_ = r
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte(h.Svc.metrics.RenderPrometheus()))
}

func (h Handler) submitBundle(w http.ResponseWriter, r *http.Request) {
	if err := h.requireAuth(r); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.Svc.cfg.MaxPayloadBytes)
	defer r.Body.Close()
	var req model.JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rec, err := h.Svc.submitWithIdentity(r.Context(), req, clientIdentity(r.RemoteAddr), h.Svc.cfg.RegionID)
	if err != nil {
		http.Error(w, err.Error(), statusForError(err))
		return
	}
	h.writeJSON(w, model.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]string{
			"bundleId": rec.ID,
			"state":    string(rec.State),
		},
	})
}

func (h Handler) requireAuth(r *http.Request) error {
	token := h.Svc.cfg.APIAuthToken
	if token == "" {
		return nil
	}
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return ErrMissingAuthorization
	}
	want := "Bearer " + token
	if header != want {
		return ErrInvalidAuthorization
	}
	return nil
}

func (h Handler) getBundle(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	rec, ok, err := h.Svc.Get(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	h.writeJSON(w, rec)
}

func (h Handler) writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func clientIdentity(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil && host != "" {
		return host
	}
	if remoteAddr != "" {
		return remoteAddr
	}
	return "unknown"
}
