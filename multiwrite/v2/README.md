# multiwrite v2

`multiwrite` v2 evolves the v1 write-path investigation into a settlement journal.

v1 studies how records move through writer strategies, buffering, syscall behavior, page cache, durability modes, and environment boundaries. v2 applies those findings to an append-only journal where correctness properties matter as much as throughput.

## Why a Settlement Journal

A settlement system needs an ordered, replayable record of events that can survive process failure, partial writes, duplicate delivery, and deployment drift.

The journal exists to provide:

- Append-only settlement event recording.
- Deterministic replay.
- Durable commit boundaries.
- Partial-write detection.
- Corruption detection.
- Idempotency under retry.
- Sequence integrity.
- Recovery to the last valid committed record.

The journal is not just a log file. It is the audit and recovery substrate for settlement state.

## How v1 Feeds v2

v1 determines the write-path tradeoffs that v2 must encode deliberately:

- Unbuffered writes versus buffered writes.
- Logical record size versus syscall count.
- Buffer size versus throughput plateau.
- Flush and close cost.
- Sync policy cost.
- Page-cache acceptance versus durable storage.
- Local, Docker Compose, and k3s environment differences.
- Profiling evidence for CPU, allocations, GC, and syscall behavior.

v2 uses those results to choose:

- Record encoding.
- Batch size.
- Checksum strategy.
- Commit marker strategy.
- Sync policy.
- Replay rules.
- Segment size.
- Recovery behavior.

## v2 Contract

- Every journal record has a stable sequence.
- Every journal record has an idempotency key.
- Every committed record can be replayed deterministically.
- Partial records are detected during replay.
- Corrupt records are detected during replay.
- Recovery stops at the last valid commit boundary.
- Duplicate delivery does not create duplicate settlement effects.
- Throughput claims include durability and recovery semantics.

## Docs

- [Design](./Design.md): settlement journal design using the project design hierarchy: problem, requirements, capacity planning, non-functional requirements, decoded reality, entities, structure, design, deep dives, and operations.

## Relationship to v1

v1 remains the performance engineering lab.

v2 is the settlement-oriented system built from the evidence v1 produces.

```text
v1: writer strategy -> kernel behavior -> durability cost -> performance boundary
v2: journal record -> commit protocol -> replay -> settlement recovery
```
