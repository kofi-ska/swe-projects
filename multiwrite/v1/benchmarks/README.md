# Benchmarks

This directory holds benchmark inputs and generated evidence for `multiwrite` v1.

The benchmark code should live with the Go package as `*_test.go` files. This directory is for reproducible run material and artifacts.

## Layout

- `configs/`: checked-in benchmark run definitions.
- `results/`: benchmark summaries intended for comparison.
- `artifacts/outputs/`: generated output files.
- `artifacts/manifests/`: run manifests emitted by the CLI.
- `artifacts/profiles/`: CPU and memory profiles.
- `artifacts/traces/`: Go execution traces.
- `artifacts/syscalls/`: syscall traces from tools such as `strace`, `dtruss`, or `fs_usage`.

## Rule

A benchmark result is only comparable when the run config, payload generation, writer strategy, durability policy, binary version, and environment envelope are recorded.

## Commands

From `multiwrite/v1`:

```bash
go test ./... -bench=. -benchmem
```

```bash
go run ./cmd/multiwrite run \
  --writer=direct \
  --profile=fixed \
  --records=100000 \
  --max-bytes=67108864 \
  --max-duration=30s \
  --record-size=32 \
  --seed=101 \
  --out=benchmarks/artifacts/outputs/local-direct-fixed.txt \
  --manifest=benchmarks/artifacts/manifests/local-direct-fixed.json \
  --durability=closed \
  --overwrite=replace
```

```bash
go run ./cmd/multiwrite run \
  --writer=buffered \
  --profile=fixed \
  --records=100000 \
  --max-bytes=67108864 \
  --max-duration=30s \
  --record-size=32 \
  --buffer-size=16384 \
  --seed=101 \
  --out=benchmarks/artifacts/outputs/local-buffered-fixed.txt \
  --manifest=benchmarks/artifacts/manifests/local-buffered-fixed.json \
  --durability=closed \
  --overwrite=replace
```

```bash
docker compose run --rm multiwrite-smoke
```

Compose workload, resource, and artifact parameters are defined in `../.env` relative to this directory.
