package service

import (
	"encoding/json"
	"net/http"
	"strings"

	"txpool-builder/v2/internal/model"
)

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/health":
		s.handleHealth(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/ready":
		s.handleReady(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/status":
		s.handleStatus(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/build":
		s.handleBuild(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/jobs/"):
		s.handleJob(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/results/"):
		s.handleResult(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Service) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"healthy": true,
		"mode":    "running",
	})
}

func (s *Service) handleReady(w http.ResponseWriter, _ *http.Request) {
	st := s.Status()
	if st.SnapshotID == "" || st.QueueDepth >= s.cfg.QueueSize {
		writeJSON(w, http.StatusServiceUnavailable, st)
		return
	}
	if st.SnapshotAgeMS > s.cfg.MaxSnapshotAge.Milliseconds() {
		writeJSON(w, http.StatusServiceUnavailable, st)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Service) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.Status())
}

func (s *Service) handleBuild(w http.ResponseWriter, r *http.Request) {
	var req model.BuildRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid request body", "detail": err.Error()})
		return
	}
	resp, code, err := s.Submit(r.Context(), req)
	if err != nil {
		writeJSON(w, code, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, code, resp)
}

func (s *Service) handleJob(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/jobs/")
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, ok := s.jobs[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "job not found"})
		return
	}
	writeJSON(w, http.StatusOK, j)
}

func (s *Service) handleResult(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/results/")
	s.mu.RLock()
	defer s.mu.RUnlock()
	res, ok := s.results[id]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "result not found"})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}
