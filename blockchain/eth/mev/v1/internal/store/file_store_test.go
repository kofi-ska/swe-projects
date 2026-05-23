package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mevrelayv1/internal/model"
)

func TestFileStorePersistAndReload(t *testing.T) {
	dir := t.TempDir()

	fs, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	rec, err := fs.Create(context.Background(), model.BundleRecord{
		ID:        "bundle-1",
		BundleHash:"0xabc",
		Request:   model.BundleRequest{Txs: []string{"0x1"}, BlockNumber: "0x1"},
		ClientID:  "client-a",
		State:     model.StateReceived,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := fs.Transition(context.Background(), rec.ID, model.StateReceived, model.StateValidated, "validated"); err != nil {
		t.Fatal(err)
	}

	auditPath := filepath.Join(dir, "audit.jsonl")
	if data, err := os.ReadFile(auditPath); err != nil {
		t.Fatal(err)
	} else if len(data) == 0 {
		t.Fatalf("expected audit log to contain events")
	}

	loaded, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	got, ok, err := loaded.Get(context.Background(), rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("expected record to reload")
	}
	if got.State != model.StateValidated {
		t.Fatalf("unexpected state: %s", got.State)
	}
}
