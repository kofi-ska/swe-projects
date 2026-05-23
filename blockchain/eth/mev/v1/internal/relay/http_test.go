package relay

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mevrelayv1/internal/model"
)

func TestHTTPHandlers(t *testing.T) {
	svc, _, cancel := newTestService(t, 8, 1, 1, "ok")
	defer cancel()

	h := Handler{Svc: svc}

	t.Run("health and ready", func(t *testing.T) {
		for _, path := range []string{"/healthz", "/readyz"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Fatalf("%s: expected 200, got %d", path, rr.Code)
			}
		}
	})

	t.Run("invalid json rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/relay/v1/bundle", strings.NewReader("{"))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("payload limit enforced", func(t *testing.T) {
		limitSvc, _, cancelLimit := newTestService(t, 8, 1, 1, "ok")
		defer cancelLimit()
		limitSvc.cfg.MaxPayloadBytes = 32
		limitHandler := Handler{Svc: limitSvc}
		large := `{"jsonrpc":"2.0","id":1,"method":"eth_sendBundle","params":[{"txs":["` + strings.Repeat("0", 32) + `"],"blockNumber":"0x1"}]}`
		req := httptest.NewRequest(http.MethodPost, "/relay/v1/bundle", strings.NewReader(large))
		rr := httptest.NewRecorder()
		limitHandler.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for oversized body, got %d", rr.Code)
		}
	})

	t.Run("submit status metrics", func(t *testing.T) {
		body := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      11,
			"method":  "eth_sendBundle",
			"params": []map[string]interface{}{
				{
					"txs":         []string{"0x1", "0x2"},
					"blockNumber":  "0x1",
					"minTimestamp": 0,
					"maxTimestamp": 0,
				},
			},
		}
		payload, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/relay/v1/bundle", bytes.NewReader(payload))
		req.Header.Set("X-Client-ID", "client-http")
		req.RemoteAddr = "10.0.0.1:4321"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d", rr.Code)
		}

		var decoded model.JSONRPCResponse
		if err := json.NewDecoder(rr.Body).Decode(&decoded); err != nil {
			t.Fatal(err)
		}
		result, ok := decoded.Result.(map[string]interface{})
		if !ok {
			t.Fatalf("expected result map, got %#v", decoded.Result)
		}
		id, _ := result["bundle_id"].(string)
		if id == "" {
			t.Fatalf("missing bundle id")
		}

		rec := waitForState(t, svc.store, id, model.StateCompleted, time.Second)
		if rec.ClientID != "client-http@10.0.0.1" {
			t.Fatalf("expected composite client identity, got %s", rec.ClientID)
		}

		req = httptest.NewRequest(http.MethodGet, "/relay/v1/bundle?id="+id, nil)
		rr = httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 for status, got %d", rr.Code)
		}

		req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rr = httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 for metrics, got %d", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), "mev_relay_received") {
			t.Fatalf("expected metrics output")
		}
	})

	t.Run("readyz reflects backend and queue state", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected ready, got %d", rr.Code)
		}

		blockingSvc, _, cancelBlocking := newTestService(t, 1, 0, 1, "ok")
		defer cancelBlocking()
		blockingHandler := Handler{Svc: blockingSvc}

		body := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      21,
			"method":  "eth_sendBundle",
			"params": []map[string]interface{}{
				{
					"txs":         []string{"0x1"},
					"blockNumber":  "0x1",
					"minTimestamp": 0,
					"maxTimestamp": 0,
				},
			},
		}
		payload, _ := json.Marshal(body)
		submitReq := httptest.NewRequest(http.MethodPost, "/relay/v1/bundle", bytes.NewReader(payload))
		submitReq.Header.Set("X-Client-ID", "client-ready")
		submitRR := httptest.NewRecorder()
		blockingHandler.ServeHTTP(submitRR, submitReq)
		if submitRR.Code != http.StatusAccepted {
			t.Fatalf("expected first submit accepted, got %d", submitRR.Code)
		}

		blockReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		blockRR := httptest.NewRecorder()
		blockingHandler.ServeHTTP(blockRR, blockReq)
		if blockRR.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected not ready when queue is saturated, got %d", blockRR.Code)
		}
	})

	t.Run("readyz fails when backend is down", func(t *testing.T) {
		downSvc, _, cancelDown := newTestService(t, 8, 1, 1, "pingfail")
		defer cancelDown()
		downHandler := Handler{Svc: downSvc}

		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rr := httptest.NewRecorder()
		downHandler.ServeHTTP(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected not ready when backend is down, got %d", rr.Code)
		}
	})
}
