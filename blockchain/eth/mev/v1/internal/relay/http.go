package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"mevrelayv1/internal/model"
)

// Handler exposes the relay HTTP API.
type Handler struct {
	Svc *Service
}

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/relay/v1/bundle":
		h.submit(w, r)
	case r.Method == http.MethodGet && (r.URL.Path == "/relay/v1/bundle" || strings.HasPrefix(r.URL.Path, "/relay/v1/bundle/")):
		h.status(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/healthz":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case r.Method == http.MethodGet && r.URL.Path == "/readyz":
		h.ready(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/metrics":
		h.metrics(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h Handler) submit(w http.ResponseWriter, r *http.Request) {
	var req model.JSONRPCRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, h.Svc.cfg.MaxPayloadBytes)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	clientID := clientIdentity(r)

	rec, err := h.Svc.Submit(r.Context(), req, clientID)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "queue overflow") || strings.Contains(err.Error(), "client inflight limit") {
			status = http.StatusTooManyRequests
		}
		writeJSON(w, status, model.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: map[string]string{
				"message": err.Error(),
			},
		})
		return
	}

	writeJSON(w, http.StatusAccepted, model.JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"bundle_id":   rec.ID,
			"state":       rec.State,
			"bundle_hash": rec.BundleHash,
			"submitted_at": time.Now().UTC(),
		},
	})
}

func (h Handler) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 250*time.Millisecond)
	defer cancel()
	if err := h.Svc.Ready(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not ready", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h Handler) status(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" && strings.HasPrefix(r.URL.Path, "/relay/v1/bundle/") {
		id = strings.TrimPrefix(r.URL.Path, "/relay/v1/bundle/")
	}
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
		return
	}

	rec, ok, err := h.Svc.Get(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, rec)
}

func (h Handler) metrics(w http.ResponseWriter, r *http.Request) {
	recs, snap, err := h.Svc.Snapshot(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	var b strings.Builder
	for k, v := range snap {
		fmt.Fprintf(&b, "mev_relay_%s %d\n", k, v)
	}
	fmt.Fprintf(&b, "mev_relay_bundles %d\n", len(recs))
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte(b.String()))
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func clientIdentity(r *http.Request) string {
	headerID := strings.TrimSpace(r.Header.Get("X-Client-ID"))
	remote := remoteIP(r.RemoteAddr)
	switch {
	case headerID == "" && remote == "":
		return "anonymous"
	case headerID == "":
		return remote
	case remote == "":
		return headerID
	default:
		return headerID + "@" + remote
	}
}

func remoteIP(addr string) string {
	if addr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.String()
	}
	return host
}
