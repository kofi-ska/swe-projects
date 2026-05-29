package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFileAndStrictUnknown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"rpc_url":"http://geth:8545","output_path":"`+filepath.Join(dir, "out.json")+`","trace_output_path":"`+filepath.Join(dir, "trace.json")+`","snapshot_output_path":"`+filepath.Join(dir, "snap.json")+`","timeout":"5s","max_transactions":2,"max_gas":1000,"max_snapshot_txs":10,"max_raw_snapshot_bytes":1000,"max_artifact_bytes":1000,"max_trace_bytes":1000,"policy_version":"v1","chain_id":"1","strict":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load([]string{"--config", path})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RPCURL != "http://geth:8545" || cfg.PolicyVersion != "v1" {
		t.Fatalf("unexpected config: %+v", cfg)
	}

	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"rpc_url":"http://geth:8545","output_path":"`+filepath.Join(dir, "out.json")+`","trace_output_path":"`+filepath.Join(dir, "trace.json")+`","snapshot_output_path":"`+filepath.Join(dir, "snap.json")+`","timeout":"5s","max_transactions":2,"max_gas":1000,"max_snapshot_txs":10,"max_raw_snapshot_bytes":1000,"max_artifact_bytes":1000,"max_trace_bytes":1000,"policy_version":"v1","chain_id":"1","strict":true,"extra_key":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load([]string{"--config", bad}); err == nil {
		t.Fatal("expected unknown key failure")
	}
}
