# TxPool Builder v1

CLI-only txpool snapshot builder for a local or single-node Geth endpoint.

## What it does
- checks RPC health at startup
- fetches `txpool_content`
- decodes `pending` transactions into typed models
- applies pre-execution eligibility checks
- ranks and selects under `max_transactions` and `max_gas`
- writes one candidate artifact, one trace artifact, and one snapshot artifact
- supports offline replay from a saved snapshot
- supports candidate comparison with `--compare-candidate`

## What it does not do
- it does not prove execution success
- it does not guarantee canonical inclusion
- it does not run distributed coordination
- it does not submit blocks
- it does not use a database

## Run

```bash
go run ./cmd/builder \
  --rpc-url http://geth:8545 \
  --output ./out/candidate.json \
  --trace-output ./out/trace.json \
  --snapshot-output ./out/snapshot.json \
  --timeout 10s \
  --max-transactions 50 \
  --max-gas 30000000 \
  --max-snapshot-txs 50000 \
  --max-raw-snapshot-bytes 10000000 \
  --max-artifact-bytes 10000000 \
  --max-trace-bytes 10000000 \
  --policy-version v1 \
  --chain-id 1
```

## Replay

```bash
go run ./cmd/builder \
  --replay-snapshot ./out/snapshot.json \
  --no-write \
  --compare-candidate ./out/candidate.json \
  --timeout 10s \
  --max-transactions 50 \
  --max-gas 30000000 \
  --max-snapshot-txs 50000 \
  --max-raw-snapshot-bytes 10000000 \
  --max-artifact-bytes 10000000 \
  --max-trace-bytes 10000000 \
  --policy-version v1
```

## Config
- `--config` loads JSON config.
- precedence is flag > env > config file > defaults.
- strict mode rejects unknown config keys.

## Tests
- `go test ./...`

## Spec
- [`docs/spec.md`](docs/spec.md)
