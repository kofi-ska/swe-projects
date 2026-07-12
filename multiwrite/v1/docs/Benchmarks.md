# Benchmarks

## Scope

v1 benchmarks the v0 write path.

Primary comparison:

- `direct`: logical record writes go directly to `os.File`.
- `buffered`: logical record writes go through `bufio.Writer`.

Path:

```text
records -> writer -> Go runtime -> write syscalls -> page cache -> filesystem
```

## Benchmark Directory

Benchmark run material lives under `benchmarks/`.

- `benchmarks/configs/`: run definitions.
- `benchmarks/results/`: comparison summaries.
- `benchmarks/artifacts/outputs/`: generated output files.
- `benchmarks/artifacts/manifests/`: run manifests.
- `benchmarks/artifacts/profiles/`: CPU and memory profiles.
- `benchmarks/artifacts/traces/`: Go execution traces.
- `benchmarks/artifacts/syscalls/`: syscall traces.

Go microbenchmarks live under `tests/` and import the engine from `src/`, so `go test -bench=. -benchmem ./...` runs them directly.

Benchmark classes:

- writer microbenchmarks use prebuilt payloads and exclude generator, manifest, CLI, and temp-file commit logic.
- generator benchmarks measure payload construction only.
- engine lifecycle benchmarks call `Run(config)` and include validation, generation, temp artifact, flush, close, commit, and manifest handling.
- Compose runs verify bounded reproducibility and artifact production.

## Hypotheses

- Buffered writes improve throughput for small records.
- Buffered writes reduce `write()` syscall count.
- Buffered writes increase average `write()` syscall size.
- Buffered writes reduce system CPU per byte.
- Buffer-size gains plateau.
- Sync, generator cost, GC, page cache, or storage can dominate.

## Optimisation Workstreams

Each optimisation must be tested against `direct` and `buffered` baselines. An optimisation is accepted only when the result includes throughput, allocation, CPU, syscall, durability, and reproducibility evidence.

### Buffer Size

Mechanism:

- Larger buffers coalesce more logical writes into fewer kernel writes.

Measure:

- `write()` syscall count.
- Average bytes per `write()`.
- `ns/op`, `B/op`, and `allocs/op`.
- Records/s and MiB/s.
- Flush and close duration.

Boundary:

- Benefit should flatten when syscall overhead stops dominating.
- Larger buffers can increase memory footprint and flush latency without improving throughput.

Disproof:

- Throughput does not improve for small records.
- Syscall count falls but total duration does not improve.
- Flush or memory cost offsets write-loop gains.

### Record Construction

Mechanism:

- Per-record string concatenation can allocate or increase CPU cost before bytes reach the writer.

Measure:

- Allocation count.
- Allocation bytes.
- CPU profile in generator and formatting paths.
- Escape analysis for record and newline handling.

Variants:

- `word + "\n"`.
- Prebuilt records including newline.
- Byte slice records.
- Fixed-size payload buffer reused per record where semantics allow.

Boundary:

- Construction cost matters when records are small and the writer is no longer the dominant cost.

Disproof:

- Allocation and CPU profiles show construction is not material.
- Kernel or storage cost dominates across tested record sizes.

### Writer Interface Shape

Mechanism:

- Interface dispatch, abstraction shape, and inlining can affect tight loops.

Measure:

- Escape analysis.
- Inlining report.
- `go test -bench=. -benchmem`.
- CPU profile for call overhead in the write loop.

Variants:

- Concrete writer type.
- Interface-based writer.
- Function closure.
- `io.Writer` adapter.

Boundary:

- Interface cost should only matter when records are small and the sink path is cheap.

Disproof:

- Compiler inlines the hot path or measured overhead is below noise.

### Batching

Mechanism:

- Application-level batches can reduce call overhead and make direct writes closer to buffered writes.

Measure:

- Batch size.
- Batch fill ratio.
- Records per batch.
- Write calls per record.
- Memory retained per batch.

Variants:

- No batching.
- Fixed record-count batch.
- Fixed byte-size batch.
- Flush on duration bound.

Boundary:

- Large batches may improve throughput but increase latency, memory pressure, and partial-failure surface.

Disproof:

- Batching duplicates `bufio` benefit without improving measured outcomes.
- Batch memory or flush latency violates bounds.

### Durability Policy

Mechanism:

- `close`, `sync`, and temp-file commit measure different durability boundaries.

Measure:

- Write-loop duration.
- Flush duration.
- Sync duration.
- Close duration.
- Commit duration.
- Artifact state after failure.

Variants:

- Close only.
- File sync before close.
- Temp file plus rename commit.

Boundary:

- Buffered write gains may disappear when sync or storage writeback dominates.

Disproof:

- Direct and buffered strategies converge under the same durability mode.
- Sync dominates total duration across record sizes.

### File Allocation and Open Policy

Mechanism:

- File creation, overwrite behavior, and optional preallocation can affect filesystem work and failure timing.

Measure:

- Startup validation time.
- File creation time.
- Write-loop time.
- Filesystem errors and artifact state.
- Disk-free validation result.

Variants:

- Direct final-path create.
- Temp-path create and commit.
- Existing-output reject.
- Existing-output replace.
- Preallocation where supported.

Boundary:

- Preallocation may help large outputs but is filesystem-dependent and must not be assumed portable.

Disproof:

- Preallocation or commit policy changes semantics more than performance.
- Filesystem support is absent or inconsistent across environments.

### Generator Cost

Mechanism:

- If generating records is expensive, writer differences become secondary.

Measure:

- Generator-only benchmark.
- Writer-only benchmark using prebuilt payloads.
- Combined end-to-end benchmark.
- CPU profile split by generator and writer.

Variants:

- Fixed payload.
- Dictionary payload.
- High-entropy payload.
- Variable-length payload.

Boundary:

- Generator cost dominates when CPU time in generation exceeds writer and syscall time.

Disproof:

- Writer remains the dominant CPU or latency cost under all profiles.

### GC and Memory Pressure

Mechanism:

- Allocation-heavy record construction or batching can move the bottleneck to GC.

Measure:

- `B/op`.
- `allocs/op`.
- Heap profile.
- GC count.
- GC pause time.
- `GODEBUG=gctrace=1` output for CLI runs.

Variants:

- String records.
- Byte slice records.
- Reused buffer.
- Fresh allocation per record.

Boundary:

- Allocation reductions matter when GC appears in profiles or runtime metrics.

Disproof:

- GC remains negligible relative to syscall, filesystem, or storage time.

### Concurrency

Mechanism:

- Producer and writer concurrency can hide generation cost but can also create queueing, scheduling, and backpressure effects.

Measure:

- Queue depth.
- Queue wait duration.
- Producer blocked duration.
- Writer service time.
- Goroutine profile.
- Block profile.
- Scheduler trace.

Variants:

- Single producer, direct writer.
- Single producer, queued writer.
- Multiple producers, bounded queue, single writer.

Boundary:

- Concurrency must not reorder records unless the workload explicitly permits it.
- Queue memory and wait time must stay bounded.

Disproof:

- Scheduler, queue, or lock overhead exceeds any generation/write overlap benefit.

## Variables

Controlled:

- generator
- seed
- record count
- byte count
- record size distribution
- durability mode
- environment
- CPU limit
- memory limit
- filesystem or volume type

Independent:

- writer strategy
- buffer size
- record size distribution
- byte count
- generator complexity
- sync policy
- environment

## Workloads

- `fixed`
- `tiny`
- `wide`
- `heavy-tail`
- `high-entropy`
- `low-entropy`
- `burst`

## Durability Modes

- `closed`: flush and close complete.
- `synced`: file sync complete.
- `committed`: temp artifact committed to final path.

## Run Metadata

Record for every run:

- command
- git commit or binary version
- Go version
- OS
- architecture
- environment
- writer strategy
- workload
- seed
- record count
- byte count
- record size distribution
- buffer size
- durability mode
- artifact state
- terminal reason
- total duration
- flush/close duration
- records/s
- MiB/s
- allocation metrics
- profile paths
- syscall trace path

## Go Benchmarks

Command:

```bash
go test ./... -bench=. -benchmem
```

Record:

- `ns/op`
- `B/op`
- `allocs/op`

Benchmarks:

- `BenchmarkDirectWriter`
- `BenchmarkBufferedWriter`
- `BenchmarkBufferedWriterSizes`
- `BenchmarkRecordSizes`
- `BenchmarkGeneratorCost`
- `BenchmarkEngineLifecycleNoSync`

Rules:

- Same prebuilt payload bytes for writer microbenchmarks.
- Same generator seed and profile for engine lifecycle benchmarks.
- Actual payload bytes are used for benchmark byte accounting.
- Same output semantics.
- Same durability mode.

## Escape Analysis

Commands:

```bash
go build -gcflags="-m" ./...
go build -gcflags="-m -m" ./...
```

Inspect:

- record formatting
- newline handling
- writer interfaces
- generator output
- buffer construction
- inlining

## Profiles

Commands:

```bash
go test ./... -bench=. -cpuprofile ./artifacts/profiles/cpu.pprof
go test ./... -bench=. -memprofile ./artifacts/profiles/mem.pprof
go tool pprof ./artifacts/profiles/cpu.pprof
go tool pprof ./artifacts/profiles/mem.pprof
```

Record:

- CPU hot spots
- heap profile
- alloc profile
- generator cost
- formatting cost
- writer cost
- runtime cost

## Runtime Diagnostics

Commands:

```bash
GODEBUG=gctrace=1 multiwrite run <args>
GODEBUG=schedtrace=1000 multiwrite run <args>
go test -race ./...
go tool trace ./artifacts/traces/run.trace
```

Record:

- GC count
- GC time
- heap growth
- goroutine count
- scheduler trace
- runtime trace
- block profile
- mutex profile
- race detector output

## Syscall Observation

Tools:

- macOS: `dtruss`, `fs_usage`
- Linux / Docker Compose: `strace`, `perf`, `/usr/bin/time -v`

Record:

- `write()` syscall count
- average bytes per `write()`
- syscall duration
- system CPU
- context switches
- I/O wait

## Environment Comparison

Environments:

- local
- Docker Compose
- k3s

Record:

- OS
- architecture
- Go version
- CPU limit
- memory limit
- filesystem
- volume kind
- container image
- storage class, for k3s
- PVC size, for k3s

Docker Compose smoke command:

```bash
docker compose run --rm multiwrite-smoke
```

Docker Compose direct/buffered comparison:

```bash
docker compose --profile benchmark up --build --abort-on-container-exit
```

Compose parameters are loaded from `.env`:

- image
- CPU and memory envelope
- mounted artifact directories
- smoke workload
- direct/buffered comparison workload
- durability and overwrite policy

## Comparison Rules

Must match unless named as the independent variable:

- git commit or binary version
- Go version
- workload
- seed
- record count
- byte count
- record size distribution
- durability mode
- resource envelope
- writer strategy
- buffer size
- sync policy
- environment

## Result Record

- hypothesis
- independent variable
- controlled fields
- command
- manifest path
- benchmark output path
- profile paths
- syscall trace path
- terminal reason
- result
- interpretation
- reproduction command

## Conclusion

```text
Workload:
Environment:
Comparison:
Observed effect:
Mechanism:
Bottleneck:
Boundary:
Evidence:
```
