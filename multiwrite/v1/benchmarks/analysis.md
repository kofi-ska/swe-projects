# Benchmark Analysis

Date: 2026-07-22

Environment:

- OS/arch: `darwin/arm64`
- Go: `go1.25.4`
- CPU reported by benchmark: `Apple M4 Pro`
- Run class: local CLI profiling run
- Durability: `closed`
- Workload: `fixed`
- Record size: 4096 bytes
- Records: 65536
- Bytes per writer: 268435456 bytes, 256 MiB
- Seed: `20260722`
- Buffered writer buffer size: 16384 bytes

## Validity Limits

This is a first profiling pass, not a completed benchmark campaign.

Limits:

- Sample count is 1 per writer for the profiled CLI run.
- CPU profile, heap profile, and Go trace were enabled in the same process.
- `/usr/bin/time` covers full process execution, including profiling setup and profile output.
- Heap profiles are `inuse_space` after a forced GC, not total allocation pressure.
- Go trace syscall profiles report syscall blocking time, not syscall count.
- Kernel syscall count was not collected.
- Flamegraph SVGs were not generated.
- Durability was `closed`, not `synced` or `committed`.
- Runs were local macOS/APFS, not Linux, Docker Compose, or k3s.

Consequence:

- This run can support a bounded claim about observed local write-path timing.
- This run cannot support a production claim about durable throughput, exact syscall reduction, storage saturation, or cross-environment behavior.

## Performance Read

The profiled CLI run says the current v1 workload is mostly a write-path experiment, not a generator experiment.

Observed dominant cost:

- Direct CPU profile: `syscall.syscall` was 93.33% of CPU samples.
- Buffered CPU profile: `syscall.syscall` was 93.41% of CPU samples.
- Direct Go trace syscall delay: 1065.09 ms total, 1053.29 ms in `syscall.write`.
- Buffered Go trace syscall delay: 902.87 ms total, 888.68 ms in `syscall.write`.
- Direct scheduler delay: 18.76 ms.
- Buffered scheduler delay: 20.43 ms.

Ratio:

- Direct syscall delay divided by direct scheduler delay: 56.77x.
- Buffered syscall delay divided by buffered scheduler delay: 44.19x.

Interpretation:

- The bottleneck in the profiled run is syscall blocking, not scheduler latency, app locks, queueing, or generator CPU.
- Since durability is `closed`, this is page-cache/filesystem acceptance behavior, not durable media throughput.
- The run is testing user-space buffering against the kernel/file path more than it is testing disk hardware.

## Throughput Decomposition

The manifest reports time inside `Run`, excluding final manifest write timing from the throughput denominator.

Direct:

- Bytes: 268435456.
- Manifest duration: 1.069307375s.
- Write loop: 1.068303250s.
- Flush and close: 0.000543584s.
- Commit rename: 0.000139625s.
- Write loop share of manifest duration: 99.906%.
- Flush and close share: 0.051%.
- Commit share: 0.013%.

Buffered:

- Bytes: 268435456.
- Manifest duration: 0.910093584s.
- Write loop: 0.909315667s.
- Flush and close: 0.000418708s.
- Commit rename: 0.000131667s.
- Write loop share of manifest duration: 99.915%.
- Flush and close share: 0.046%.
- Commit share: 0.014%.

Interpretation:

- Optimization pressure should stay on the write loop.
- Final flush/close is not where buffered gained in this workload.
- The buffered writer is flushing during the loop as its internal 16 KiB buffer fills.
- Rename commit is below measurement significance for this local workload.

## Amortization Model

Logical workload:

- Record size: 4096 bytes.
- Record count: 65536.
- Total bytes: 256 MiB.

Expected output write calls, ignoring partial writes and metadata writes:

| writer | expected output `write()` calls | bytes per full output write | records per full output write |
| --- | ---: | ---: | ---: |
| direct | 65536 | 4096 | 1 |
| buffered | 16384 | 16384 | 4 |

Expected reduction:

- Buffered should reduce output write calls by about 75%.
- Buffered should increase average output write size from 4 KiB to 16 KiB.

Measured proxy:

- `/usr/bin/time` system CPU fell from 0.22s to 0.08s, a 63.636% reduction.
- Go trace `syscall.write` delay fell from 1053.29 ms to 888.68 ms, a 15.628% reduction.
- Manifest duration fell from 1.069307s to 0.910094s, a 14.889% reduction.

Interpretation:

- The system CPU reduction is directionally consistent with syscall coalescing.
- The wall-time gain is much smaller than the expected write-call reduction because syscall count is not the only cost.
- Filesystem acceptance, page-cache state, copy cost, profiling overhead, and run-to-run noise can dominate after the most obvious syscall overhead is removed.

## CLI Run Result

| writer | manifest duration_s | real_s | user_s | sys_s | MiB/s | records/s | write_loop_s | flush_close_ms | commit_ms |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| direct | 1.069307 | 1.52 | 0.87 | 0.22 | 239.407 | 61288.271 | 1.068303 | 0.544 | 0.140 |
| buffered | 0.910094 | 1.01 | 0.85 | 0.08 | 281.290 | 72010.177 | 0.909316 | 0.419 | 0.132 |

Measured effect:

- Buffered manifest throughput was 1.174942x direct throughput.
- Buffered manifest duration was 14.889% lower than direct.
- Buffered `/usr/bin/time` system CPU was 63.636% lower than direct.
- User CPU was approximately flat: direct 0.87s, buffered 0.85s.
- Flush and commit were not material in this workload: both were under 1 ms per writer.

Interpretation:

- The measured improvement is in the write loop, not in commit or close.
- Buffered flush cost mostly appears inside the write loop because `bufio.Writer.Write` flushes full buffers as records are written.
- Lower system CPU is consistent with fewer kernel transitions, but this local run did not obtain syscall counts.
- The run used `closed` durability, so it measures completion of flush, close, and rename policy, not durable storage commit.
- Because the profiled CLI run has one sample per writer, the measured difference is an observation, not a distribution.

Artifacts:

- Direct manifest: `benchmarks/artifacts/manifests/profile-direct-256m-4k.json`
- Buffered manifest: `benchmarks/artifacts/manifests/profile-buffered-256m-4k.json`
- Direct CPU profile: `benchmarks/artifacts/profiles/profile-direct-256m-4k.cpu.pprof`
- Buffered CPU profile: `benchmarks/artifacts/profiles/profile-buffered-256m-4k.cpu.pprof`
- Direct heap profile: `benchmarks/artifacts/profiles/profile-direct-256m-4k.mem.pprof`
- Buffered heap profile: `benchmarks/artifacts/profiles/profile-buffered-256m-4k.mem.pprof`
- Direct trace: `benchmarks/artifacts/traces/profile-direct-256m-4k.trace`
- Buffered trace: `benchmarks/artifacts/traces/profile-buffered-256m-4k.trace`

## CPU Profile

Direct CPU profile:

- Total samples: 1.05s over 1.21s.
- `syscall.syscall`: 0.98s flat, 93.33%.
- `DirectFileWriter.WriteRecord`: 0.98s cumulative through `os.File.Write`.
- `RecordGenerator.Next`: 0.03s cumulative, 2.86%.

Buffered CPU profile:

- Total samples: 0.91s over 1.01s.
- `syscall.syscall`: 0.85s flat, 93.41%.
- `BufferedFileWriter.WriteRecord`: 0.85s cumulative through `bufio.Writer.Write` and `bufio.Writer.Flush`.
- `RecordGenerator.Next`: 0.01s cumulative, 1.10%.

Interpretation:

- Both profiled CLI runs are syscall-dominated.
- Generator cost is not the bottleneck for fixed 4 KiB records in this run.
- Buffered reduced total sampled syscall time from 0.98s to 0.85s, a 13.27% reduction.
- CPU profile shape supports the hypothesis that the difference is in the write path.
- CPU profiles include profiling and trace overhead because CPU profiling and runtime tracing were enabled together.
- The profile is still useful for hot-path location, but not a clean standalone CPU-cost measurement.

Performance implication:

- There is little evidence that optimizing random generation would improve this specific 4 KiB fixed-profile run.
- There is evidence that changing write strategy changes runtime behavior.
- If the next CPU-only profile still shows more than 90% in syscall paths, further Go-level micro-optimizations outside the writer will have low expected value.

## Heap Profile

Direct heap profile:

- In-use heap: 5577.09 KiB.
- `runtime/trace.(*traceMultiplexer).startLocked`: 3525.87 KiB, 63.22%.
- `runtime.allocm`: 1539 KiB, 27.60%.
- `runtime.malg`: 512.22 KiB, 9.18%.

Buffered heap profile:

- In-use heap: 6636.45 KiB.
- `runtime/trace.(*traceMultiplexer).startLocked`: 2880.57 KiB, 43.41%.
- `runtime/pprof.StartCPUProfile`: 1184.27 KiB, 17.84%.
- `runtime.procresize`: 1033.53 KiB, 15.57%.
- `runtime.allocm`: 1026 KiB, 15.46%.
- `compress/gzip.NewWriterLevel`: 512.08 KiB, 7.72%.

Interpretation:

- Heap retention is dominated by profiling and runtime machinery.
- The configured 16 KiB buffered writer is too small to dominate an in-use heap profile.
- The heap profiles do not show record construction as the main retained-memory cost after GC.
- These heap profiles do not measure transient allocation volume; use `-benchmem` and alloc-space profiles for that.

Performance implication:

- Memory retention is not the measured bottleneck in this run.
- Allocation rate still matters because `generator.Next` allocates one `[]byte` per record.
- For this run, that implies 65536 record allocations before profiler/runtime allocations are considered.
- A reusable record buffer could reduce allocation pressure, but it would only help if allocation or GC becomes visible in CPU, alloc-space, or trace data.

## Go Trace Profiles

Syscall blocking profile:

| writer | total syscall delay_ms | `syscall.write` delay_ms | `syscall.write` share |
| --- | ---: | ---: | ---: |
| direct | 1065.09 | 1053.29 | 98.89% |
| buffered | 902.87 | 888.68 | 98.43% |

Measured effect:

- Buffered reduced trace-observed `syscall.write` delay by 15.628%.
- Both traces are dominated by write syscall blocking.

Scheduler latency profile:

| writer | scheduler delay_ms | top material non-runtime item |
| --- | ---: | --- |
| direct | 18.76 | `math/rand.(*Rand).Intn`, 3.81 ms cumulative |
| buffered | 20.43 | `math/rand.(*Rand).Intn`, 6.47 ms cumulative |

Interpretation:

- Scheduler delay is small relative to syscall delay.
- There is no evidence from this trace that scheduling latency explains the direct/buffered difference.
- The trace was collected with profiling enabled, so scheduler numbers should be treated as diagnostic direction, not standalone latency truth.

Performance implication:

- Adding goroutines or queues is not supported by this result.
- The current default path is single-threaded and syscall-blocked.
- Parallel generation could help only if generator CPU becomes material.
- Multiple concurrent writers to one ordered output would add ordering and synchronization risk without evidence of benefit for this workload.

Sync blocking profile:

- Direct sync profile was dominated by runtime trace machinery, not application locks.
- Buffered sync profile reported 66.37 us in `runtime.gcMarkDone`.
- The default path has no queue, worker pool, semaphore, or application mutex contention.

## Flamegraphs

Attempted command:

```bash
go tool pprof -svg -output=benchmarks/artifacts/profiles/profile-direct-256m-4k.cpu.svg benchmarks/artifacts/profiles/profile-direct-256m-4k.cpu.pprof
```

Result:

- Flamegraph generation failed because Graphviz `dot` was not installed in `PATH`.
- CPU profiles were still collected and interpreted through `go tool pprof -top`.
- No SVG flamegraph artifact exists for this run.

## Syscall Tracing

Attempted local syscall tool:

```bash
dtruss -f -t write
```

Result:

- `dtruss` failed because DTrace lacked required macOS privileges.
- `strace` was not available locally.
- `fs_usage` was available but was not used as a substitute for counted `write()` calls.

Consequence:

- This run has syscall blocking evidence from Go trace.
- This run does not have kernel syscall counts.
- The claim that buffering reduced `write()` call count remains expected but unproven for this run.
- The expected syscall count for the output artifact alone is approximately 65536 direct writes versus approximately 16384 buffered writes for 4 KiB records and a 16 KiB buffer, assuming full writes and no partial-write retries.
- That expectation must be confirmed with kernel tracing before it becomes a measured result.

Performance implication:

- Without syscall counts, the mechanism is not closed.
- The strongest current kernel-level statement is about syscall blocking time, not call frequency.
- Docker/Linux with `strace -c -e write` is the next decisive measurement.

## Go Benchmarks

Command:

```bash
GOCACHE=/Users/kofiamediku/Documents/Github/kofi-ska/swe-projects/multiwrite/v1/.gocache go test ./... -bench=. -benchmem
```

Writer microbenchmarks:

| benchmark | ns/op | MB/s | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| `BenchmarkDirectWriter` | 3851624 | 34.03 | 0 | 0 |
| `BenchmarkBufferedWriter` | 61059 | 2146.63 | 0 | 0 |

Measured effect:

- Buffered writer microbenchmark was 63.080x faster than direct.
- Both writer microbenchmarks had zero benchmarked allocations.

Benchmark regime:

- Writer microbenchmarks use 4096 prebuilt records per benchmark operation.
- The direct and buffered writer microbenchmarks shown here use 32-byte records.
- Each writer microbenchmark operation writes about 128 KiB, not 256 MiB.
- Writer construction is outside the timed region.
- Flush and close are inside the timed region.
- Payload generation is outside the timed region.

Consequence:

- The 63.080x microbenchmark result is not comparable to the 1.174942x CLI profiling result.
- The microbenchmark isolates syscall amortization for many tiny records.
- The CLI profiling run measures a much larger artifact, larger records, generation, commit, manifest handling, and profiler overhead.

Performance implication:

- Buffering has its largest measured advantage when records are tiny and syscall overhead dominates.
- The full CLI gain is smaller because 4 KiB records already amortize some syscall cost and the filesystem path has other costs.
- Claims about large artifacts must come from CLI runs, not only `go test -bench`.

Buffered buffer-size sweep:

| buffer size | ns/op | MB/s | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| 4 KiB | 117701 | 1113.60 | 0 | 0 |
| 16 KiB | 65487 | 2001.49 | 0 | 0 |
| 64 KiB | 48333 | 2711.83 | 0 | 0 |
| 256 KiB | 47204 | 2776.71 | 0 | 0 |

Measured effect:

- 4 KiB to 16 KiB reduced `ns/op` by 44.362%.
- 16 KiB to 64 KiB reduced `ns/op` by 26.195%.
- 64 KiB to 256 KiB reduced `ns/op` by 2.336%.
- The plateau begins by 64 KiB in this benchmark.

Performance implication:

- 16 KiB is materially better than 4 KiB in the microbenchmark.
- 64 KiB is materially better than 16 KiB in the microbenchmark.
- 256 KiB is barely better than 64 KiB in the microbenchmark.
- A reasonable next CLI sweep should test 16 KiB, 64 KiB, and 256 KiB; it should not assume 256 KiB is worth the extra memory.

Record-size sweep:

| record size | ns/op | MB/s | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| 8 B | 42114 | 778.08 | 0 | 0 |
| 32 B | 62958 | 2081.90 | 0 | 0 |
| 128 B | 151910 | 3451.30 | 0 | 0 |
| 1 KiB | 1171952 | 3578.90 | 0 | 0 |
| 4 KiB | 4264803 | 3933.88 | 0 | 0 |

Interpretation:

- Throughput rises with record size because fixed per-write overhead is amortized across more bytes.
- Allocation count remains zero in the writer benchmark path.
- This record-size sweep is buffered-only; it does not prove direct and buffered convergence by record size.

Performance implication:

- The buffering advantage should be largest for small records.
- With larger records, each syscall carries more bytes, so buffering should matter less.
- This must be verified with paired direct and buffered record-size sweeps.

Generator benchmarks:

| profile | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| fixed | 215.9 | 64 | 1 |
| tiny | 29.70 | 8 | 1 |
| wide | 13181 | 4096 | 1 |
| heavy-tail | 870.0 | 265 | 1 |
| high-entropy | 220.2 | 64 | 1 |
| low-entropy | 29.48 | 64 | 1 |
| burst | 936.2 | 287 | 1 |

Interpretation:

- Generator cost is profile-dependent and allocates once per generated record.
- For the 4 KiB fixed CLI run, CPU profiles show generator cost below write syscall cost.
- For heavier profiles, generator cost must be separated from writer cost before claiming write-path gains.
- Generator benchmarks measure generation in isolation and do not include file writes.

Performance implication:

- `wide` generation costs 13181 ns/op and allocates 4096 B/op in isolation.
- `tiny` generation costs 29.70 ns/op and allocates 8 B/op.
- The generator can become the bottleneck when writer cost is reduced or when the profile is expensive.
- A settlement-journal workload should not benchmark only synthetic random payload construction; it should separate serialization cost from append cost.

Full write-path and engine lifecycle:

| benchmark | writer | ns/op | MB/s | B/op | allocs/op |
| --- | --- | ---: | ---: | ---: | ---: |
| `BenchmarkWritePathNoSync` | direct | 3934142 | 33.32 | 0 | 0 |
| `BenchmarkWritePathNoSync` | buffered | 62774 | 2088.01 | 0 | 0 |
| `BenchmarkEngineLifecycleNoSync` | direct | 4914332 | 26.67 | 146691 | 4136 |
| `BenchmarkEngineLifecycleNoSync` | buffered | 757947 | 172.93 | 163087 | 4137 |

Measured effect:

- Buffered write-path benchmark was 62.672x faster than direct.
- Buffered engine lifecycle benchmark was 6.484x faster than direct.
- Lifecycle benchmark allocations come from full run setup, generation, temp artifact, commit, and manifest path, not from the writer microbenchmark.

Benchmark caveat:

- The engine lifecycle benchmark writes 4096 records of 32 bytes each, about 128 KiB per operation.
- It is useful for lifecycle overhead, not for large-file throughput.
- Its allocation count is not directly comparable to the 256 MiB CLI run.

Performance implication:

- Lifecycle overhead compresses the measured improvement from 62.672x in writer-only to 6.484x in engine lifecycle.
- Even in a small lifecycle run, buffering is still material.
- Manifest and artifact handling matter more at small byte counts; they should become less material as bytes per run increases.
- For v2 journal work, segment lifecycle overhead must be measured separately from append-loop throughput.

## Escape Analysis

Command:

```bash
GOCACHE=/Users/kofiamediku/Documents/Github/kofi-ska/swe-projects/multiwrite/v1/.gocache go build -gcflags=-m ./...
```

Relevant findings:

- `writer.go`: direct writer `record does not escape`.
- `writer.go`: buffered writer `record` leaks to the writer call path.
- `writer.go`: `bufio.NewWriterSize` allocates `bufio.Writer` and its backing byte slice on the heap.
- `generator.go`: `make([]byte, size)` escapes to heap.
- `generator.go`: `rand.Rand` and source state escape as part of generator construction.
- `engine.go`: result manifest encoding causes `result` to escape.
- `cmd/main.go`: CLI flag storage and profiling closure escape.

Interpretation:

- Writer-only benchmarks report zero allocations despite escape-analysis notes in setup paths.
- Generator allocation is real and expected: one generated payload allocation per record.
- The benchmark path must keep generator-only and writer-only measurements separate.
- Escape analysis explains possible allocation sites; benchmark allocation counters decide whether those sites are exercised in the timed region.

Performance implication:

- The hot writer path is not currently allocation-bound in microbenchmarks.
- Generator allocation is the obvious Go-level allocation target.
- The buffered writer's heap allocation is fixed per writer construction, not per record.
- Reducing per-record allocation is more likely to help future journal payload handling than tuning writer construction.

## Bottleneck Classification

Current observed bottleneck for the profiled CLI case:

```text
write syscall blocking > generator CPU > scheduler delay > flush/close > rename commit
```

Evidence:

- Write syscall delay was approximately 1 second.
- Scheduler delay was approximately 20 ms.
- Flush and close were below 1 ms.
- Commit was below 0.2 ms.
- Generator CPU was below 3% of CPU samples in both CPU profiles.

What this means:

- The current direct-vs-buffered comparison is meaningful because both writers spend most time at the write boundary.
- The result does not yet locate the next bottleneck after buffering; the likely candidates are page-cache behavior, filesystem behavior, storage writeback, and generator allocation.
- The next benchmark campaign should force the bottleneck to move by changing record size, buffer size, durability, and environment.

## v2 Journal Implications

For a settlement journal, this result implies:

- Append batching is likely useful for small journal entries.
- A single append per event can waste system CPU if events are small.
- Buffering alone is insufficient for correctness; v2 must define commit boundaries, checksums, sequence numbers, idempotency, and replay behavior.
- `closed` durability is not enough for settlement correctness when the journal is the source of truth.
- `synced` or explicit segment commit policy will likely reduce the apparent buffering advantage.
- Segment size and flush policy should be tuned from measured append latency, recovery cost, and durability requirements, not throughput alone.
- The journal should expose separate metrics for append accepted, bytes buffered, bytes flushed, sync duration, segment commit duration, replay duration, and partial segment recovery.

Expected v2 performance questions:

- What is the maximum append rate per ordered partition under a selected durability policy?
- What is p50, p95, and p99 append latency for small entries?
- How many entries are lost or replayed under crash at each commit boundary?
- At what batch size does throughput improve without violating settlement latency constraints?
- At what segment size does recovery time become unacceptable?
- Does idempotency lookup become the bottleneck before append I/O?
- Does checksum computation become material for small or large payloads?
- Does replay saturate CPU, disk, or deserialization?

## Attacks On The Result

The result is fragile under these conditions:

- If `synced` durability dominates, buffering may stop mattering.
- If record generation becomes high-entropy or wide, CPU can move from writes to generation.
- If storage writeback throttling starts, wall duration may be controlled by filesystem and page-cache behavior.
- If the run is repeated after large prior writes, APFS/cache state can change results.
- If the buffer size is near or above the plateau, larger buffers may add memory without measurable speedup.
- If the benchmark sink is faster than real storage, microbenchmarks can overstate gains.
- If profiles and traces are collected together, memory and syscall profiles include observability overhead.
- If `bufio.Writer` behavior changes across Go versions, buffer-size results need revalidation.

Missing evidence before accepting a strong claim:

- repeated profiled runs with medians and spread
- kernel `write()` counts
- alloc-space profiles
- independent CPU-only and trace-only runs
- `synced` durability comparison
- Linux or Docker Compose comparison
- CLI-level buffer-size sweep
- CLI-level record-size sweep across both direct and buffered writers

## Conclusion

For one 256 MiB local macOS run of fixed 4 KiB records with `closed` durability:

- Buffered writing was faster by 17.494% on manifest throughput.
- Buffered writing reduced system CPU by 63.636%.
- CPU profiles were syscall-dominated for both writers.
- Go trace syscall profiles showed lower write syscall blocking for buffered.
- Heap profiles were dominated by profiling/runtime machinery, not records or writer buffers.
- Writer microbenchmarks showed a much larger buffered advantage than the full CLI run, so full-run results must not be inferred from microbenchmarks alone.
- Syscall count was not collected because local DTrace was blocked and `strace` was unavailable.
- Flamegraph SVGs were not generated because Graphviz `dot` was unavailable.

The current evidence supports:

```text
buffering improved this local workload by reducing observed time in the write path
```

The current evidence does not yet prove:

```text
buffering reduced kernel write syscall count by a specific amount
buffering improves durable write throughput
the measured speedup is stable across repeated profiled runs
```

Required next run:

- Docker/Linux syscall trace with `strace -c -e write`.
- Flamegraph generation in an environment with Graphviz.
- Same comparison under `synced` durability.
- Buffer-size sweep using CLI manifests, not only Go microbenchmarks.
- Separate CPU-only, heap-only, and trace-only runs.
