# v1 Spec

## Problem

Build a bounded compute-offload system for Solana-adjacent work.

Goal:
- take transaction-like requests
- decide them once
- offload only the expensive part
- keep the system measurable, auditable, and bounded

The offload boundary is not fixed here.

Transport options:
- FFI
- JNI
- gRPC
- REST

Selection rule:
- use the cheapest boundary that preserves the required failure isolation, deployability, and cost profile

Current reality:
- v0 was not FFI
- v0 was in-process Scala logic only
- v0 did not measure cross-runtime cost

## Adversarial reality

Assume every request can be:
- duplicated
- replayed
- delayed
- reordered
- malformed
- oversized
- retried after partial failure

Assume every subsystem can:
- stall
- crash
- back up
- return stale state
- produce partial writes

Assume load is bursty, not smooth.

## Functional Requirements

The system must:
- accept requests through a Scala orchestration layer
- normalize each request before decision
- dedupe by request identity or equivalent idempotency key
- classify each request into an outcome
- offload only when offload cost is justified
- preserve deterministic outcomes for the same normalized input and state
- bound retries, queue depth, and memory growth
- emit a decision record for every request

Required outcomes:
- accept
- reject_duplicate
- reject_malformed
- reject_risk
- defer
- fail_closed

## Constraints

Hard constraints:
- no unbounded queue
- no unbounded retry loop
- no silent duplicate side effects
- no unbounded memory growth
- no dependence on Solana consensus internals
- no claim that offload changes on-chain execution rules

Boundary constraints:
- every boundary crossing has cost
- every serialization step has cost
- every retry multiplies cost
- every failure mode must be visible

Operational constraints:
- `ρ <= 0.7` in normal load
- `Q_max` finite
- `W_max` finite
- `R_max` finite

## Qualities

The system should be:
- bounded
- idempotent
- auditable
- deterministic for identical inputs
- failure-aware
- backpressure-aware
- cost-aware
- measurable

Quality targets:
- duplicate input does not create duplicate side effects
- overload degrades into rejection or deferment, not collapse
- failures stay local
- boundaries are justified by economics, not preference

## Targets

Throughput target:
- v2: `1,000,000 ops/hour`
- `1,000,000 / 3600 = 277.78 ops/sec`

Sizing implication:
- if a stage cannot sustain `277.78 ops/sec` at `ρ <= 0.7`, it is a bottleneck

Per-request targets must be measured:
- CPU/request
- allocations/request
- memory/request
- I/O/request
- boundary overhead/request
- duplicate hit rate
- replay rejection rate
- recovery time

Economic target:
- offload only when `compute_saved > boundary_overhead + validation_overhead + failure_risk_cost`

## Problem View

### State machine

Model the system as a deterministic state machine with bounded concurrency around it.

Core states:
- `received`
- `normalized`
- `deduped`
- `classified`
- `queued`
- `dispatched`
- `computing`
- `persisted`
- `audited`
- `completed`
- `rejected`
- `deferred`
- `failed`

Terminal states:
- `completed`
- `rejected`
- `deferred`
- `failed`

Transition rules:
- every transition has a guard
- every transition has a measurable cost
- every transition is reversible only if explicitly modeled
- identical normalized input + identical state snapshot must choose the same terminal state

Guard examples:
- `received -> normalized` if payload parses
- `normalized -> deduped` if request ID / dedupe key is present
- `deduped -> classified` if not already completed
- `classified -> queued` if work is worth continuing
- `queued -> dispatched` if capacity exists
- `dispatched -> computing` if boundary admission succeeds
- `computing -> persisted` if compute succeeds
- `persisted -> audited` if durable write succeeds
- `audited -> completed` if final record is emitted

Failure transitions:
- parse failure -> `rejected`
- duplicate hit -> `rejected`
- policy failure -> `rejected`
- overload -> `deferred`
- boundary failure -> `failed`
- compute failure -> `failed`
- persistence failure -> `failed`
- audit failure -> `failed`

State machine design:
- input enters once
- every request carries a request ID and dedupe key
- normalize before any expensive work
- dedupe before queue admission
- classify before dispatch
- dispatch only if capacity and policy both allow it
- commit only after compute, persist, and audit all succeed
- terminal state is chosen once and recorded once
- any ambiguous transition fails closed

### Graph

The graph is the transition graph of the state machine:
- `V = {ingress, parse, normalize, dedupe, risk, queue, actor, marshal, boundary, worker, persist, audit, response}`
- `E = transitions`

Per-edge metrics:
- latency `l(e)`
- queue delay `q(e)`
- allocation delta `a(e)`
- memory delta `m(e)`
- bytes serialized `b(e)`
- failure probability `p(e)`
- retry probability `r(e)`

Path cost:

`C(path) = Σ(wl·l + wq·q + wa·a + wm·m + wb·b + wp·p + wr·r)`

Critical cut sets:
- boundary
- queue
- persistence
- audit

Graph objective:
- minimize weighted path cost
- minimize variance
- minimize duplicate side effects
- keep cut sets recoverable

State-machine objective:
- minimize time spent in non-terminal states
- minimize failed-transition count
- minimize ambiguity in transition choice
- ensure every request reaches exactly one terminal state

### Queue

For a stage:
- arrival rate `λ`
- service rate per worker `μ`
- worker count `c`
- utilization `ρ = λ / (cμ)`

Rules:
- target `ρ <= 0.7`
- degraded `ρ <= 0.85` only briefly
- `Q_max` finite
- `W_max` finite
- `R_max` finite

Burst buffer:

`Q_max >= max(0, (λ_b - cμ) * t_r)`

Sizing:

`c >= ceil((λ * s) / ρ_target)`

where `s` is mean service time per request.

Queue objective:
- absorb bursts without collapse
- reject before memory pressure becomes failure
- keep queueing as a controlled state, not an implicit backlog

### Information

Goal:
- reduce uncertainty per request

Required fields:
- request ID
- dedupe key
- payload hash
- partition key
- risk class
- schema version
- timestamp or monotonic nonce where needed

Decision rule:
- if the request cannot be classified deterministically, reject or defer

Information rule:
- same normalized input + same state snapshot -> same terminal outcome
- duplicate request -> cached outcome or duplicate rejection
- replay must not create new state

Entropy objective:
- minimize decision ambiguity
- minimize re-evaluation of identical state
- minimize missing-information retries

State-machine information rule:
- each transition must have enough information to choose one next state
- if the next state is not decidable, reject or defer
- do not carry hidden state in the transport

## High-Level Design

Orchestration:
- Scala actors own request flow, policy, retries, and state transitions
- state machine is the source of truth for terminal outcome
- queue is explicit and bounded
- audit is appended for every decision

Compute:
- Rust owns bounded expensive work
- offload happens only after normalization, dedupe, and classification
- transport is chosen by deployment model, not ideology

Boundary choice:
- FFI/JNI if same-process latency matters and failure coupling is acceptable
- gRPC if this becomes a real service boundary
- REST only if request size and latency budget are coarse enough

Scaling rule:
- the system scales by bounding work, not by accepting more backlog
- overload must become defer or reject, not hidden queue growth

Commercial rule:
- offload only when compute saved exceeds boundary, validation, and failure costs
- if that is not true, keep work in Scala

## Transport Note

The spec does not commit to one boundary transport yet.

Use:
- FFI if same-process and the boundary cost is lower than the operational risk
- JNI if JVM interop is required and the same-process tradeoff is acceptable
- gRPC if service isolation, versioning, and independent deployment matter more
- REST only if the workload is coarse enough that HTTP overhead is acceptable

Rule:
- if the deployment model is microservice-like, gRPC is the default
- if the deployment model is in-process library-like, FFI/JNI may be acceptable

## Low-Level Design

Data structures:

- request envelope
  - fields: request ID, dedupe key, payload hash, schema version, risk class, timestamps
  - cost: `O(1)` field access, `O(n)` serialization size where `n` is payload length
  - lifecycle: create at ingress, discard after terminal state is logged

- dedupe store
  - use: hash map / key-value store with TTL
  - cost: `O(1)` average lookup/insert, `O(n)` hash on key length
  - lifecycle: insert on first sight, expire by replay window, compact periodically

- bounded queue
  - use: ring buffer or fixed-capacity queue
  - cost: `O(1)` enqueue/dequeue
  - lifecycle: admit only when capacity exists, drop/defer on overflow

- actor mailbox
  - use: bounded mailbox per actor or shard
  - cost: `O(1)` enqueue/dequeue, plus scheduling overhead
  - lifecycle: holds in-flight control messages only, never unbounded request backlog

- audit log
  - use: append-only log
  - cost: `O(1)` append amortized, storage growth proportional to request rate
  - lifecycle: write once per terminal decision, retain by policy

Algorithms:

- normalization
  - canonicalize request fields
  - validate schema and required keys
  - cost: linear in payload size

- deduplication
  - hash request identity
  - check TTL-based store
  - cost: linear in key size, constant average lookup

- classification
  - rule-based or score-based policy check
  - cost: bounded by policy complexity

- scheduling
  - bounded queue admission
  - actor routing by key/partition
  - cost: constant per enqueue/dequeue plus contention overhead

- offload decision
  - compare predicted compute saved against boundary cost
  - cost: constant, but decision inputs must be measured

- persistence/audit
  - append decision and outcome
  - cost: dominated by storage and durability policy

LLD lifecycle:
- construct request envelope
- normalize and validate
- dedupe check
- classify
- enqueue or defer
- dispatch to transport
- run compute
- persist result
- audit decision
- expire request state after TTL or checkpoint

LLD cost model:
- CPU per stage must be measured
- allocation per stage must be measured
- serialization bytes must be measured
- queue residency must be bounded
- storage writes must be amortized
- garbage retention must be time-bounded

LLD rule:
- if a structure or algorithm raises hot-path allocation without lowering state uncertainty or boundary cost, it is too expensive
