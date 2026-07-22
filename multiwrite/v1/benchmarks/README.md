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

Wall-clock duration is not enough evidence. A direct-vs-buffered result must also show where the time went.

Required evidence for a complete comparison:

- benchmark output with `ns/op`, `B/op`, and `allocs/op`
- run manifest for each writer
- CPU profile summary for each writer
- heap profile summary for each writer
- flamegraph for each CPU profile
- Go execution trace summary for scheduler, syscall, and GC behavior
- syscall summary with `write()` count, total bytes, average write size, and system CPU
- interpretation that ties application timing to kernel behavior

Expected result shape:

- `direct` should perform more kernel writes for small records.
- `buffered` should reduce kernel writes and increase average write size.
- `buffered` should reduce system CPU per byte when syscall overhead dominates.
- Direct and buffered results should converge when records are large or durability/storage dominates.
- Larger buffers should eventually stop improving throughput.

## Commands

From `multiwrite/v1`:

```bash
go test ./... -bench=. -benchmem
```

```bash
go test ./... -bench=. -benchmem \
  -cpuprofile benchmarks/artifacts/profiles/bench.cpu.pprof \
  -memprofile benchmarks/artifacts/profiles/bench.mem.pprof
```

```bash
go tool pprof -top benchmarks/artifacts/profiles/bench.cpu.pprof
go tool pprof -top benchmarks/artifacts/profiles/bench.mem.pprof
go tool pprof -svg benchmarks/artifacts/profiles/bench.cpu.pprof
```

```bash
go run ./src/cmd/multiwrite run \
  --writer=direct \
  --profile=fixed \
  --records=100000 \
  --max-bytes=67108864 \
  --max-duration=30s \
  --record-size=32 \
  --seed=101 \
  --out=benchmarks/artifacts/outputs/local-direct-fixed.txt \
  --manifest=benchmarks/artifacts/manifests/local-direct-fixed.json \
  --cpuprofile=benchmarks/artifacts/profiles/local-direct-fixed.cpu.pprof \
  --memprofile=benchmarks/artifacts/profiles/local-direct-fixed.mem.pprof \
  --trace=benchmarks/artifacts/traces/local-direct-fixed.trace \
  --durability=closed \
  --overwrite=replace
```

```bash
go run ./src/cmd/multiwrite run \
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
  --cpuprofile=benchmarks/artifacts/profiles/local-buffered-fixed.cpu.pprof \
  --memprofile=benchmarks/artifacts/profiles/local-buffered-fixed.mem.pprof \
  --trace=benchmarks/artifacts/traces/local-buffered-fixed.trace \
  --durability=closed \
  --overwrite=replace
```

```bash
go tool trace benchmarks/artifacts/traces/local-direct-fixed.trace
go tool trace benchmarks/artifacts/traces/local-buffered-fixed.trace
```

Syscall tracing is platform-specific. Store raw traces under `artifacts/syscalls/` and summarize:

- `write()` count
- total bytes written
- average bytes per `write()`
- syscall duration
- system CPU
- context switches, when available

Linux example:

```bash
strace -c -e write ./benchmarks/artifacts/multiwrite run <args>
```

macOS examples:

```bash
dtruss -f -t write ./benchmarks/artifacts/multiwrite run <args>
fs_usage -w -f filesystem ./benchmarks/artifacts/multiwrite
```

```bash
docker compose run --rm multiwrite-smoke
```

Compose workload, resource, and artifact parameters are defined in `../.env` relative to this directory.
