# Lookahead

This document states the target shape of the work, not the current implementation boundary.

## North Star

The goal is a parallel pattern library for Erlang on MIMD machines that lets developers express structured parallel computation while preserving correctness, bounded coordination cost, fault isolation, and measurable speedup. The library should make parallelism reusable without hiding the cost model: task granularity, data movement, process scheduling, message passing, load imbalance, memory pressure, and failure handling must remain explicit enough to reason about.

In plain terms, the library is not a set of concurrency helpers. It is a contract for turning problem structure into predictable parallel execution.

## Why this matters

Parallelism only wins when the work is large enough to pay for coordination. If tasks are too small, serialization dominates, or workers are badly imbalanced, the runtime does not create speedup; it creates overhead. This project exists to keep that tradeoff visible.

- Correctness must preserve task identity, result identity, ordering semantics, failure semantics, and completion semantics.
- Cost must account for dispatch, serialization, copying, worker count, mailbox depth, scheduler pressure, memory footprint, and retry cost.
- Resources must be bounded: workers, inflight tasks, buffering, retries, and queue growth.
- Failure handling must be part of the pattern contract, not an optimistic add-on.
- Performance claims must be measured against a sequential baseline and a custom parallel baseline.

## Economic objective

Maximize useful speedup per unit of coordination cost, memory overhead, implementation complexity, and debugging burden.

This is the central constraint. Parallelism is not free. It is a loan, and the interest is coordination, copying, scheduling, imbalance, and failure recovery.

## Pattern rent

Each pattern must justify itself against the same economic test.

- `farm` pays rent when work units are independent, similar in shape, and large enough to beat dispatch overhead.
- `pipeline` pays rent when work has ordered stages and throughput improves by overlapping stage execution.
- `divide-and-conquer` pays rent when a problem splits cleanly and combine cost does not dominate.
- `reduce` pays rent when aggregation is associative or deterministic enough to parallelize safely.
- `stencil` or data-parallel work pays rent when computation is local and communication boundaries are controlled.

If the work is tiny, sequential wins.

If copying dominates, sequential wins.

If workers are imbalanced, parallelism lies.

If ordering constraints are too strong, parallelism collapses into theatre.

## Non-negotiable forces

### Correctness

Parallel execution must not introduce races, lost messages, duplicated work, missed outputs, nondeterministic corruption, or invalid ordering.

Erlang removes shared-memory races, but it does not remove concurrency bugs. Messages can arrive out of order, workers can crash, retries can duplicate work, coordinators can bottleneck, mailboxes can grow unbounded, and pipeline stages can deadlock through backpressure.

### Cost model

The runtime must expose the cost of parallelism rather than conceal it.

Track at minimum:

- task execution time
- dispatch overhead
- serialization and copying cost
- worker count
- mailbox depth
- scheduler pressure
- data size per task
- stage throughput
- stage imbalance
- combine or reduce cost
- memory footprint
- failure and retry cost

### Bounded resources

The library must bound:

- number of workers
- inflight tasks
- mailbox growth or queue depth
- input buffering
- output buffering
- retry count
- stage queues
- intermediate data size
- result accumulation
- scheduler pressure

### Fault isolation

Failure handling must be explicit.

If one worker crashes, should the task retry? If one stage dies, should the pipeline replay? If one node dies, is the work retried, failed, or reclassified? These answers belong in the pattern contract.

### Performance truth

The library only succeeds if reuse does not destroy the speedup it abstracts.

Benchmarks should compare:

- sequential baseline
- custom parallel implementation
- library-based parallel implementation

And measure:

- wall-clock time
- speedup
- efficiency
- worker utilization
- queue depth
- mailbox growth
- memory usage
- scheduler utilization
- throughput
- stage latency
- load imbalance
- overhead versus computation time

## v1 alignment

v1 should carry this north star forward as an implementation contract:

- bounded lifecycle
- explicit pattern choice
- measurable recovery
- durable state
- observable execution
- local-first deployment on K3s

That makes v1 a runtime with a clear economic model, not just a benchmark harness with nicer diagrams.
