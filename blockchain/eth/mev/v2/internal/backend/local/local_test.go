package local

import (
	"context"
	"testing"

	"mevrelayv2/internal/model"
)

func TestLocalAdapter(t *testing.T) {
	a := New()
	if err := a.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	res, err := a.Simulate(context.Background(), model.BundleRecord{
		Request: model.BundleRequest{Txs: []string{"0x1", "0x2"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Success {
		t.Fatalf("expected success, got %#v", res)
	}
}
