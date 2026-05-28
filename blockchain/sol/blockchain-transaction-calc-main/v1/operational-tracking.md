# Operational Tracking

Aligned to:
- [`REALITY.md`](./REALITY.md)
- [`tokenomics.md`](./tokenomics.md)

## Invariants

- every request has exactly one `request_id`
- every request has exactly one `dedupe_key`
- every terminal request has exactly one durable decision row
- every terminal request has exactly one append-only audit trail
- every state transition is attributable to one request
- every recorded EV value is tied to a request, model version, and source hash set
- `EV_lower_bound > 0` is required for accept
- stale or conflicting source state cannot produce a terminal accept
- duplicate requests cannot create duplicate terminal side effects
- replay must resolve from durable state before recomputation
- audit coverage for terminal requests is complete
- observability coverage for lifecycle transitions is complete

## Guarantees

If the invariants hold, the system guarantees:
- one request -> one terminal outcome
- one request -> one durable decision record
- one request -> one audit trail
- replay returns recorded terminal state instead of re-executing work
- accept decisions are only made when lower-bound EV is positive
- stale or conflicting source state cannot silently pass
- duplicate requests do not advance state twice
- terminal decisions are reconstructible from request ID, source hashes, and model version
- operational health is measurable at each lifecycle stage

## Scope

Track request lifecycle, economic decisioning, replay, audit, and service health across:
- Scala orchestrator
- Rust compute
- decision store
- hot cache / dedupe cache
- audit stream

## Required identifiers

Every request, event, span, and log line carries:
- `trace_id`
- `request_id`
- `decision_id`
- `dedupe_key`
- `model_version`
- `route_id`
- `slot`
- `quote_age`
- `source_hashes`
- `service_name`
- `service_version`
- `terminal_state`
- `reason_code`

## Lifecycle tracking

| Lifecycle stage | What to record | Where it is written | Why it is recorded |
|---|---|---|---|
| Ingress | request receipt, headers, payload size, auth result, source service | Scala logs + trace span | start of request lineage, spoof detection, traffic accounting |
| Parse / normalize | schema version, normalized payload hash, canonical fields | Scala span + decision record draft | prove canonical input before compute |
| Dedupe check | request ID, dedupe key, cache hit/miss, TTL state | Valkey + Scala span | suppress duplicates before work |
| Classification | policy outcome, terminal candidate, reason code | Scala span + decision record draft | show why the request is accepted, deferred, or rejected |
| Queue admission | queue depth, wait estimate, admission result | Scala span + metrics | prove bounded backpressure and queue health |
| Dispatch | transport deadline, payload bytes, routing target, gRPC status | Scala span + gRPC metrics | prove boundary cost and isolate transport failure |
| Rust compute | route score, quote, slippage, fee, breakeven, EV, risk, freshness | Rust span + audit event | this is the economic heart of the system |
| Persist decision | terminal state, reason code, model version, source hashes, EV fields | PostgreSQL row | durable terminal truth and replay source |
| Audit emit | terminal event, transition event, identifiers, timestamps | Redpanda event | append-only history and downstream replay |
| Completion | terminal state, ack time, total latency | Scala span + metrics | prove end-to-end completion and latency |
| Replay / recovery | durable lookup hit, checkpoint used, terminal state restored | PostgreSQL + Scala span | recover from crash without re-executing work |

## Why each subsystem is tracked

### Scala orchestrator

Track:
- ingress rate
- parse failures
- schema validation failures
- normalization latency
- dedupe hit rate
- dedupe miss rate
- per-key contention
- actor mailbox depth
- queue depth
- queue wait time
- retry count
- defer count
- reject count
- accept count
- request lifecycle latency
- gRPC request latency
- gRPC error rate
- gRPC timeout rate
- replay lookup hit rate
- durable decision write latency
- audit publish latency
- terminal decision coverage
- stale-source rejection count
- source-conflict count

Why:
- this is the control plane
- it owns state transitions, backpressure, retries, and replay
- it must show where requests wait, fail, or terminate

### Rust compute

Track:
- request count
- compute start/end timestamps
- route scoring latency
- quote latency
- slippage latency
- fee estimation latency
- breakeven latency
- EV estimate latency
- EV lower-bound latency
- risk scoring latency
- freshness check latency
- source-conflict check latency
- batch size
- candidate route count
- actionable result rate
- non-actionable result rate
- stale-data marker rate
- conflict marker rate
- compute error rate
- panic rate
- CPU time per request
- memory per request
- serialization bytes in/out

Why:
- this is the economic compute plane
- route scoring, EV, and risk are the decision inputs
- low-level numeric cost must be visible before offload is justified

### Decision store

Track:
- insert latency
- update latency
- lookup latency
- replay hit rate
- duplicate request hit rate
- terminal-row write count
- write failure rate
- transaction rollback rate
- read-after-write latency
- row size
- storage growth
- connection pool saturation
- unique-constraint violations on `request_id`

Why:
- this is the terminal source of truth
- every durable decision must be recoverable by `request_id`
- unique constraint failures expose duplicate side effects

### Hot cache / dedupe cache

Track:
- hit rate
- miss rate
- stale entry rate
- TTL expiry rate
- write latency
- read latency
- eviction count
- replication lag
- persistence success rate
- persistence failure rate
- dedupe suppression count
- key cardinality
- hot-key skew
- cache memory usage

Why:
- this accelerates duplicate suppression
- it protects the hot path from repeated requests
- it must reveal when dedupe is drifting or too hot

### Audit stream

Track:
- publish latency
- publish success rate
- publish failure rate
- partition lag
- consumer lag
- retention age
- replay count
- event rate
- event loss count
- ordering violations
- duplicate event rate
- end-to-end audit coverage

Why:
- this is the append-only history
- it supports replay, audit, and downstream consumers
- lag or loss means the record is incomplete

## EV tracking

Track EV at three points.

| EV type | Where it is calculated | Where it is recorded | Why it is recorded |
|---|---|---|---|
| `EV_estimate` | Rust compute after quote, route scoring, fee, slippage, and risk computation | Rust span, Scala decision draft, audit event | this is the pre-trade forwarding value used for accept/defer/reject |
| `EV_lower_bound` | Rust compute after uncertainty margin is applied | Rust span, Scala decision routing, PostgreSQL decision row | this is the hard acceptance gate; if it is not positive, do not accept |
| `EV_realized` | downstream execution feedback, settlement reconciliation, or post-trade observation | PostgreSQL reconciliation row, Redpanda audit event | this is the real outcome used to test whether the model and controls are economically viable |

Record EV together with:
- `request_id`
- `decision_id`
- `model_version`
- `route_id`
- `slot`
- `quote_age`
- `source_hashes`
- `risk_score`
- `fee_cost`
- `slippage_cost`
- `compute_cost`

Why EV is tracked:
- acceptance requires positive lower-bound EV
- stale quotes can make an apparently positive EV invalid
- route scoring only matters if the route survives all penalties
- realized EV is needed to verify whether the system is profitable or just busy

## Operational health

Track:
- p50 latency per service
- p95 latency per service
- p99 latency per service
- queue depth
- worker utilization
- backlog age
- retry amplification
- timeout rate
- error rate
- audit coverage
- replay drift
- source-conflict rate
- freshness-failure rate
- model-version mismatch rate
- dead-letter count if used
- SLO burn rate
- error budget burn rate

Why:
- these show whether the system is healthy under live traffic
- they expose overload, stale data, replay bugs, and control-plane failure

## Event model

Emit structured events for:
- request received
- request normalized
- request deduped
- request classified
- request queued
- request dispatched
- request computed
- request persisted
- request audited
- request completed
- request deferred
- request rejected
- request failed

Each event includes:
- identifiers
- timestamp
- state transition
- reason code
- source hashes
- model version
- route id
- slot
- quote age
- stage latency
- bytes in
- bytes out
- success / failure flag
