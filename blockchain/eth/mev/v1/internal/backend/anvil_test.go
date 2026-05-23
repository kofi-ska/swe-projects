package backend

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"mevrelayv1/internal/model"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestAnvilBackendSuccess(t *testing.T) {
	b := NewAnvil("http://example.invalid")
	b.Client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			body := io.NopCloser(bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"result":"0x1"}`))
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       body,
			}, nil
		}),
	}

	res, err := b.Simulate(context.Background(), model.BundleRecord{
		ID: "bundle-1",
		Request: model.BundleRequest{
			Txs: []string{"0x1", "0x2"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("expected success")
	}
	if res.ProfitEth <= 0 {
		t.Fatalf("expected profit")
	}
}

func TestAnvilBackendRetryableFailure(t *testing.T) {
	b := NewAnvil("http://example.invalid")
	b.Client = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			body := io.NopCloser(bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"down"}}`))
			return &http.Response{
				StatusCode: http.StatusServiceUnavailable,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       body,
			}, nil
		}),
	}

	_, err := b.Simulate(context.Background(), model.BundleRecord{
		ID: "bundle-1",
		Request: model.BundleRequest{
			Txs: []string{"0x1"},
		},
	})
	if err == nil {
		t.Fatalf("expected retryable error")
	}
}
