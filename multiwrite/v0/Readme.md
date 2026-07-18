# multiwrite

## Overview

Investigate high-volume sequential file writing with an emphasis on I/O throughput measurement, buffering behaviour, and system-level observation.

It is designed as a controlled testbed for understanding disk write performance.

## Goals

- Measure raw write throughput under different buffering strategies. Extended via Writer Interface.
- Observe syscall behaviour and OS page cache effects
- Provide a foundation for stress-testing disk I/O paths
- Keep the core logic simple and inspectable

## Architecture (Current)

1. A single generator produces words.
2. A single buffered writer appends words to a file.
3. Writes are batched in memory using `bufio.Writer`.
4. Execution time is measured end-to-end.

## Configuration

- Buffer size configured via `BufferConfig`
- Default buffer size: 16 KB
- Output path currently hardcoded

## Usage

Run with defaults (1 million lines, default output path):

```bash
go run .
```

Output is written to:

```
output/words.txt
```

Specify the number of lines to write:

```bash
go run . -n 1000000
```

Specify both line count and output file:

```bash
go run . -n 50000000 -out output/words.txt
```

Flags
```
-n
Number of lines to write (default: 1_000_000_000)

-out
Output file path (default: output/words.txt)
```

## What This Measures Today

- End-to-end write duration
- Effect of buffering on syscall frequency
- Sustained sequential write behaviour

## Planned Work

This project is intentionally incremental. Planned additions include:

- Throughput benchmarks (MB/s, words/sec)
- Syscall tracing (strace / dtruss)
- Progress logging and ETA
- Configurable buffer sizes via flags
- Concurrent producer pipelines (single writer)
- File rotation strategies
- Streaming compression writers
- pprof profiling hooks
- Deterministic generators for repeatability
- Lightweight metrics export


## Why This Exists

- In this implementation, writing each word directly to a file would trigger a syscall per write, causing frequent user–kernel context switches

- These context switches are expensive relative to in-memory operations, and small, frequent writes quickly amplify syscall overhead

- As syscall frequency increases, the program becomes CPU-bound, making raw disk throughput irrelevant

- This implementation uses bufio.Writer to buffer data in user-space memory before writing to disk

- Buffering allows many small writes to be coalesced into fewer, larger write operations

- Larger sequential writes align better with filesystem and disk I/O characteristics

- The chosen buffer size directly affects syscall frequency and write latency variance. 

- By buffering writes, this implementation isolates true I/O behaviour and enables meaningful throughput measurement