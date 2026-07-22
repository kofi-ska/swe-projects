# Local 1 GiB Fixed 4 KiB Run

Date: 2026-07-12

Environment:

- local macOS / darwin arm64
- Go: 1.25.4
- filesystem label recorded as `apfs`
- durability: `closed`
- workload: `fixed`
- record size: 4096 bytes
- records: 262144
- bytes per run: 1073741824
- seed: 1001
- buffered writer buffer size: 16384 bytes

Important caveat:

- These runs measure kernel acceptance into the file path under `closed` durability.
- They do not prove durable disk throughput because no file sync or parent directory sync is used.
- Throughput degraded across repeated runs, so writeback, cache state, filesystem state, thermal behavior, or background I/O likely affected later samples.

Profiling coverage:

- CPU profiles were not collected.
- Heap profiles were not collected.
- Flamegraphs were not generated.
- Go execution traces were not inspected for scheduler, syscall, or GC behavior.
- Syscall traces were not collected.
- Escape analysis was not reviewed for this result.

This result is an initial throughput sample, not a complete performance investigation.

## Samples

| sample | writer | manifest | real_s | user_s | sys_s | duration_s | MiB/s | records/s | terminal_reason |
| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| 1 | direct | `benchmarks/artifacts/manifests/local-1g-direct-4k.json` | 8.25 | 7.16 | 1.13 | 8.251 | 124.106 | 31771.051 | records_exhausted |
| 2 | direct | `benchmarks/artifacts/manifests/local-1g-direct-4k-r2.json` | 10.48 | 8.08 | 1.51 | 10.472 | 97.781 | 25032.028 | records_exhausted |
| 3 | direct | `benchmarks/artifacts/manifests/local-1g-direct-4k-r3.json` | 15.96 | 8.98 | 1.63 | 15.884 | 64.466 | 16503.247 | records_exhausted |
| 1 | buffered | `benchmarks/artifacts/manifests/local-1g-buffered-4k.json` | 5.60 | 5.21 | 0.40 | 5.596 | 182.983 | 46843.588 | records_exhausted |
| 2 | buffered | `benchmarks/artifacts/manifests/local-1g-buffered-4k-r2.json` | 8.96 | 7.57 | 0.69 | 8.951 | 114.397 | 29285.631 | records_exhausted |
| 3 | buffered | `benchmarks/artifacts/manifests/local-1g-buffered-4k-r3.json` | 12.45 | 8.67 | 0.96 | 12.444 | 82.290 | 21066.227 | records_exhausted |

## Aggregate

| writer | mean real_s | median real_s | mean MiB/s | median MiB/s | mean sys_s |
| --- | ---: | ---: | ---: | ---: | ---: |
| direct | 11.56 | 10.48 | 95.451 | 97.781 | 1.423 |
| buffered | 9.00 | 8.96 | 126.557 | 114.397 | 0.683 |

## Read

Buffered was faster in every sample for this workload, but the effect was not stable:

- sample 1: buffered was about 1.47x faster by manifest MiB/s
- sample 2: buffered was about 1.17x faster by manifest MiB/s
- sample 3: buffered was about 1.28x faster by manifest MiB/s

The system CPU time was lower for buffered in every sample. That is consistent with fewer kernel write calls, but syscall count was not collected in this run, so the syscall mechanism remains inferred rather than proven.

Next evidence needed:

- Linux or Compose run with `strace` write syscall counts.
- CPU and heap profiles for direct and buffered runs.
- CPU flamegraphs for direct and buffered runs.
- Go execution traces for direct and buffered runs.
- `synced` durability run to separate page-cache acceptance from storage durability.
- Larger buffer-size sweep at 1 GiB.
- Smaller record-size run to amplify syscall overhead.
