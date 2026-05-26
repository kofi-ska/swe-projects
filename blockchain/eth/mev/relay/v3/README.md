# MEV Relay v3

v3 is a shard-local MEV relay. The public ETH relay contract stays the same. Routing, authority, queueing, recovery, and evidence move behind shard ownership.

## What this repo describes

- a relay that rejects stale or unsafe work early
- one owner per shard
- bounded retry and queue behavior
- fenced recovery and rejoin
- append-only evidence for terminal decisions
- a control stack that extends v2 with policy adaptation, controller separation, and a proof loop

## Core model

- shard key: canonical bundle ID + network ID + target slot
- ownership: a shard owns bundles; client, validator, and region are metadata
- routing: rendezvous hashing over a fixed shard set
- authority: lease, epoch, fence token
- retry rule: max `3`, `500ms` backoff, only while expected value stays positive
- chain rule: fail closed when the view is stale or inconsistent
- recovery rule: snapshot + WAL + checkpoint replay, then re-fence
- observability rule: sampled traces, bounded logs, bounded label sets
- deployment uses OTLP to an OpenTelemetry Collector, then Jaeger for trace storage and UI

## Runtime defaults

| Setting | Value | Use |
|---|---:|---|
| `QUEUE_DEPTH` | `1024` | per-shard queue cap |
| `WORKER_COUNT` | `4` | per-shard worker pool |
| `MAX_RETRIES` | `3` | retry cap |
| `RETRY_BACKOFF` | `500ms` | retry spacing |
| `LEASE_TTL` | `5s` | shard lease lifetime |
| `LEASE_RENEW_INTERVAL` | `1s` | lease renewal cadence |
| `REQUEST_TIMEOUT` | `2s` | request budget |
| `MAX_PAYLOAD_BYTES` | `256KiB` | request size cap |
| `MAX_INFLIGHT_PER_CLIENT` | `20` | client concurrency cap |
| `HISTORY_LIMIT` | `256` | retained event history |
| `STATE_RETENTION` | `24h` | state retention window |
| `WAL_MAX_ENTRIES` | `2048` | WAL compaction bound |

## Docs

- architecture: [`docs/architecture.md`](./docs/architecture.md)
- quantification: [`docs/quantification.md`](./docs/quantification.md)
- lifecycle math: [`docs/lifecycle-math.md`](./docs/lifecycle-math.md)
- v2 gaps: [`docs/v2-gaps.md`](./docs/v2-gaps.md)
- service levels: [`docs/service-levels.md`](./docs/service-levels.md)
- measurement: [`docs/measurement.md`](./docs/measurement.md)
- operations: [`docs/operations.md`](./docs/operations.md)
- recovery: [`docs/recovery.md`](./docs/recovery.md)
- security: [`docs/security.md`](./docs/security.md)
- testing: [`docs/testing.md`](./docs/testing.md)
- delivery: [`docs/delivery.md`](./docs/delivery.md)
- docs index: [`docs/README.md`](./docs/README.md)
- deployment: [`infra/compose.yaml`](./infra/compose.yaml)
- measurement runner: [`cmd/measure`](./cmd/measure)

## Operating bounds

- queue age target: under `1s`
- queue age at `1 slot` (`12s`) is unsafe
- queue full is unsafe
- stale authority is unsafe
- low chain confidence is unsafe
- retries stop when expected value is non-positive
- recovery must re-fence before rejoin



## Operational states

| State | Meaning |
|---|---|
| Ready | authority current, queue age under target, confidence above threshold |
| Degraded | pressure rising but still bounded |
| Unsafe | authority stale, queue full, confidence below threshold, or recovery inconsistent |
| Draining | stop new work, finish in-flight work, seal state, release authority |

## Public surfaces

| Endpoint | Purpose |
|---|---|
| `/relay/v1/data/validator_registration` | validator registration lookup |
| `/relay/v1/builder/validators` | builder-facing validator set |
| `/relay/v1/builder/blocks` | builder block submission |
| `/healthz` / `/readyz` | health and readiness |

## NFRs

| NFR | Target |
|---|---|
| Correctness | one owner per shard or bundle; no double terminalization |
| Safety | fail closed on uncertainty |
| Boundedness | queue, retries, retention, and scans are capped |
| Latency | cheap reject path; no global coordination on hot path |
| Throughput | shard-local workers; bounded backpressure |
| Recoverability | deterministic replay with fencing and version checks |
| Auditability | every accepted decision leaves reconstructable evidence |
| Observability | traces, metrics, logs show authority, pressure, provenance |
| Scalability | scale by sharding, not by adding global coordination |
| Operability | clear health states, clear status codes, clear failover rules |

## Data structures

Live:

- rendezvous hashing for shard routing
- lease / epoch / fence records for authority
- exact dedupe map for idempotency
- per-shard priority queue for admission and retries
- inflight map for bounded concurrency
- append-only event log for durable evidence
- Merkle trees for checkpoint sealing
- Valkey for coordination state
- MinIO for checkpoint artifacts

Offline:

- SCC and reachability for recovery and dependency validation
- min-cut / flow for capacity analysis
- replay graph checks for recovery safety

## Risk model

| Risk | Control |
|---|---|
| split-brain ownership | leases, epochs, fencing tokens |
| retry storm | max `3` retries, `500ms` backoff, EV gate |
| broker lag | idempotent consumers; broker as transport only |
| checkpoint corruption | Merkle sealing; valid checkpoint only |
| audit divergence | append-only events; bounded flush |
| queue pressure | deadline-aware admission and shedding |
| rollout overlap | version fences and cutover windows |
| observability overload | sampling, bounded cardinality |

## Status codes

| Condition | Status |
|---|---|
| accepted into bounded pipeline | `202 Accepted` |
| malformed request | `400 Bad Request` |
| unauthorized / forbidden | `401 Unauthorized` / `403 Forbidden` |
| duplicate or ownership conflict | `409 Conflict` |
| stale precondition / stale epoch | `412 Precondition Failed` |
| payload too large | `413 Payload Too Large` |
| economically invalid | `422 Unprocessable Entity` |
| rate / inflight / budget exceeded | `429 Too Many Requests` |
| unsafe state or unhealthy dependency | `503 Service Unavailable` |
| upstream timeout | `504 Gateway Timeout` |
| internal invariant failure | `500 Internal Server Error` |

## Observability

- Jaeger traces carry request ID, bundle ID, shard ID, region ID, lease ID, epoch, fence token, chain-view ID, finality depth, confidence score, recovery state, and decision outcome
- metrics include request rate, queue depth, queue age, queue net value, retry debt, worker saturation, backend latency, state latency, broker latency, decision rate, and dead-letter rate
- slow paths and failures are sampled; labels stay bounded; raw payloads are not logged by default

## Proof loop

- `cmd/measure` runs steady, burst, and faulted scenarios against the same relay stack or a live HTTP target
- reports capture health, metrics, latency, and regression flags before and after each run

## Economics

Operating cost is negative by default until value preservation or revenue is proven.

```text
ExpectedNet = ExpectedValue - DelayCost - ComputeCost - RetryCost - FailureRisk
```

Admission requires:

```text
ExpectedNet > 0
```

Reference costs:

- compute: about `$245/month`
- managed hot state: about `+$110/month`
- storage: low tens/month
- logging / trace: `0` while under free tiers
- egress: variable

Reference break-even:

- lean baseline: about `$250/month`
- managed hot state: about `$350/month`
- `10,000` accepted bundles/month: about `$0.025-$0.035` per bundle
- `100,000` accepted bundles/month: about `$0.0025-$0.0035` per bundle

## Launch gate

Do not promote v3 unless:

- public relay APIs match mev-boost expectations
- one shard has one authority at a time
- stale writers are rejected on every write path
- recovery re-fences before rejoin
- queue age stays under target in stress tests
- retry debt stays bounded under burst
- chain confidence fails closed when stale or inconsistent
- audit trails reconstruct the same terminal truth
- observability stays sampled under load
- infra cost stays inside the operating envelope
- rollouts can cut over without mixed authority

## What we are striving for

v3 is trying to make the relay behave like a governed control system, not just a queue with checks:

- every state transition is legal and observable
- authority is explicit, fenced, and recoverable
- policy changes come from measured pressure, not guesswork
- recovery and rollout are controlled paths, not side effects
- measurement is part of the runtime contract, not an afterthought
- the system preserves invariants under uncertainty and competing objectives

## Non-goals

- Envoy by default
- gRPC everywhere
- microservice sprawl
- global hot-path coordination
- unbounded replay
- active-active multi-region before authority is proven
- optimistic action on uncertain chain state
