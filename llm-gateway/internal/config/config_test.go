package config

import "testing"

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	t.Setenv("WORKER_COUNT", "0")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid worker count to fail")
	}
}

func TestLoadDefaults(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.QueueCapacity != 1000 || c.MaxBatchSize != 8 || c.WorkerCount != 8 {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}
