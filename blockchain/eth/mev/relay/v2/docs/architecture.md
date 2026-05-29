# v2 Architecture

Bounded relay. External state. Append-only evidence. Short hot path.

## Core pieces

- ingress: validate, size-limit, reject junk
- admission: score value and deadline slack
- scheduler: bounded work queue
- backend: pluggable Ethereum simulation
- state: lifecycle, inflight, retries, events, checkpoints
- broker: event and checkpoint fan-out
- health: readiness from queue, state, backend, broker, WAL
- telemetry: counters and gauges

## Invariants

- one terminal path per bundle
- stale work is rejected early
- retries are bounded
- lifecycle transitions fence state writes
- audit output is append-only
- health fails closed when a dependency or queue condition is unsafe
- transition results are reused; no extra read is needed after each state hop
- event and checkpoint payloads are encoded once and reused for WAL writes
- WAL compaction is deferred until the log is well past the configured entry bound

## Ownership

- `internal/relay` owns lifecycle orchestration
- `internal/state` owns coordination state
- `internal/eventlog` owns durable append-only evidence
- `internal/scheduler` owns bounded queueing
- `internal/backend` owns simulation adapter contracts
- `internal/broker` owns transport fan-out
