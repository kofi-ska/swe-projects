# v1 Benchmark

This benchmark measures the live pre-trade decision loop described in `REALITY.md`, `tokenomics.md`, and `operational-tracking.md`.

It is not a swap executor benchmark and not a market backtest.

The question is simple: under concurrent load, does v1 classify requests correctly, preserve durable truth, keep replay deterministic, and accept only positive lower-bound EV requests?

## Reality Model

The service is a bounded state machine.

In-flight occupancy is modeled as:

```text
N_inflight(t) = Σ_state N_state(t)
```

Throughput is bounded by the slowest stage:

```text
Throughput_max ≈ min(μ_ingress, μ_dedupe, μ_compute, μ_store, μ_audit)
```

The accept gate is:

```text
accept iff EV_lower_bound > 0
       and freshness_valid
       and source_state_valid
       and route_constraints_valid
```

`EV_estimate` is the pre-trade decision value.
`EV_lower_bound` is the accept gate.
`EV_realized` is not measured here because v1 stops at pre-trade decisioning.

## What Was Measured

Two layers were exercised:

1. Compute-only readiness smoke against the Rust gRPC boundary.
2. Full-stack benchmark through Scala orchestrator -> Rust compute -> PostgreSQL decision store -> Valkey dedupe cache -> Redpanda audit stream.

The full-stack benchmark also includes duplicate replay, single accept, and single stale scenarios.

## Compute-Only Readiness Smoke

The Rust compute service was hit directly with 100 concurrent requests.

Observed terminal split:

- `ACCEPT=70`
- `DEFER=20`
- `RESOURCE_EXHAUSTED=10`

The accepted sample responses carried positive `EV_lower_bound` values in the `~1.09` to `~1.23` range.

The sample accepted round-trip timings were in the low single-digit millisecond range, roughly `2.75 ms` to `3.12 ms`.

What this proves:

- freshness gating is active
- oversized route sets fail closed before expensive work
- the Rust boundary is concurrent and stable

What this does not prove:

- full-stack latency under store and audit pressure
- replay correctness
- durable decision persistence
- audit recovery

## Full-Stack Benchmark

The live stack was exercised through the real orchestration path.

| Scenario | Requests | Terminal split | Reasons | p50 ms | p95 ms | p99 ms | `EV_estimate` | `EV_lower_bound` | Interpretation |
|---|---:|---|---|---:|---:|---:|---|---|---|
| `burst_100_mixed` | 100 | `ACCEPT=70`, `DEFER=20`, `FAILED=10` | `ACCEPTED=70`, `STALE_QUOTE=20`, `RESOURCE_EXHAUSTED: too many route candidates=10` | 558 | 597 | 653 | `avg=0.9331466666666666666666666666666667`, `min=0.000000`, `median=1.199760`, `max=1.199760` | `avg=0.9175211111111111111111111111111111`, `min=0.000000`, `median=1.179670`, `max=1.179670` | Burst load hits queueing and durable side effects |
| `duplicate_replay` | 2 | `ACCEPT=2` | `ACCEPTED=2` | 6 | 13 | 13 | `avg=1.199760000000`, `min=1.199760`, `median=1.199760000000`, `max=1.199760000000` | `avg=1.179670000000`, `min=1.179670`, `median=1.179670000000`, `max=1.179670000000` | Duplicate replay resolves from durable truth |
| `single_accept` | 1 | `ACCEPT=1` | `ACCEPTED=1` | 4 | 4 | 4 | `avg=1.199760`, `min=1.199760`, `median=1.199760`, `max=1.199760` | `avg=1.179670`, `min=1.179670`, `median=1.179670`, `max=1.179670` | Fast-path accept |
| `single_stale` | 1 | `DEFER=1` | `STALE_QUOTE=1` | 4 | 4 | 4 | `avg=0.000000`, `min=0.000000`, `median=0.000000`, `max=0.000000` | `avg=0.000000`, `min=0.000000`, `median=0.000000`, `max=0.000000` | Stale input is terminalized cheaply |

### EV Interpretation

The accepted path in this benchmark has:

- `EV_estimate = 1.19976`
- `EV_lower_bound = 1.17967`
- uncertainty haircut `≈ 0.02009`
- haircut as a share of estimate `≈ 1.67%`

That is the key economic signal in v1:

- the model is not merely finding a positive point estimate
- it keeps the lower bound positive after uncertainty
- the accept gate is not speculative

The mixed burst average EV is lower than the accepted-path EV because the aggregate includes zero-EV stale and oversized requests that are terminalized before accept.

### Tokenomics Readout

The tokenomics layer is doing four things in this benchmark:

- it computes route viability from quote, fee, slippage, breakeven, and risk
- it sets the acceptance gate on `EV_lower_bound > 0`
- it rejects stale or oversized requests before they can consume more work
- it preserves a durable reason code for every terminal decision

What we realized from the live run is not trading P&L.
What we realized is the decision economy:

- accepted requests carried positive estimated and lower-bound EV
- stale requests were deferred before accept
- oversized route sets were rejected before expensive work
- duplicate replay returned the same durable outcome instead of generating a new one

That is the economic realization v1 is designed to prove: the system spends compute only on requests that remain admissible after uncertainty, and it refuses to forward requests that do not clear the gate.

This is also the difference between `EV_estimate`, `EV_lower_bound`, and `EV_realized` in `operational-tracking.md`:

- `EV_estimate` is the model output before uncertainty haircut
- `EV_lower_bound` is the live accept gate
- `EV_realized` is only available after downstream execution and reconciliation

## What The Results Show

- The request state machine is live and deterministic.
- Freshness gating works as a hard gate.
- Oversized requests fail closed before heavy work.
- Duplicate replay resolves from durable decision truth.
- Accept and stale decisions both travel through the same live stack.
- The measured EV lower bound stays positive on accepted paths.
- The benchmark follows the lifecycle in `operational-tracking.md`: ingress, normalize, dedupe, classify, queue, dispatch, compute, persist, audit, complete, replay.
- The benchmark realizes the decision economy described in `tokenomics.md`, not downstream execution P&L.

## Benchmark Gaps

The benchmark proves the control loop. It does not yet prove the economic envelope.

### EV gaps

- There is no sweep around `EV_lower_bound = 0`.
- There is no measurement for the negative side of the gate, such as `EV_lower_bound = -ε`.
- There is no measurement for marginal cases, such as `EV_lower_bound = +ε`.
- There is no false-positive count for accepted requests that later prove economically invalid.
- There is no false-negative count for deferred or rejected requests that would have cleared the gate.
- There is no sensitivity curve for fee, slippage, risk, or freshness against EV.
- There is no realized EV reconciliation, so the benchmark cannot tell whether positive decision EV maps to positive outcome EV.

### Distribution gaps

- The workload is shaped, not sampled from a live population.
- The benchmark does not report EV percentiles across a realistic request mix.
- The benchmark does not report route-count, hop-count, or quote-age distributions.
- The benchmark does not report decision split by EV bucket.
- The benchmark does not report latency by EV bucket.
- The benchmark does not report repeated-run variance or confidence intervals.

### Cost gaps

- The benchmark does not measure cost per decision.
- The benchmark does not measure cost per accepted decision.
- The benchmark does not measure cost per audited terminal request.
- The benchmark does not measure infra cost versus positive decision volume.
- The benchmark does not measure whether positive decision EV clears operating cost with margin.

### State-machine gaps

- The benchmark proves terminal behavior on accept, defer, duplicate replay, and stale input.
- It does not yet enumerate the full transition matrix under dependency failure.
- It does not test crash-before-write, crash-after-write, or crash-after-audit.
- It does not test replay after schema change or model version change.
- It does not test clock skew or monotonicity drift.

### Operational gaps

- The benchmark does not report queue depth, worker utilization, or backlog age during the run.
- It does not report cache hit rate or dedupe suppression rate for the benchmark scenarios.
- It does not report audit lag or publish latency.
- It does not break end-to-end latency into ingress, dedupe, compute, store, and audit components.

### Comparative gaps

- v0 has no comparable request envelope, replay contract, or EV gate.
- v1 therefore cannot be compared to v0 on execution quality.
- v0 only establishes that local map math exists; v1 establishes that a bounded decision system exists.
- The benchmark therefore compares operational decisioning, not generalized trading performance.

## Stack-Level Readout

### Rust compute

The compute-only run is the cleanest measurement of the numeric gate.

It shows:

- concurrent request handling is stable
- route limits are enforced
- stale inputs are deferred
- accepted requests carry positive `EV_lower_bound`
- compute latency is low single-digit milliseconds on the accepted sample path

This is the part of the stack that determines whether a request is economically admissible.

### Scala orchestrator + stores + audit

The full-stack run adds durable state, dedupe, and audit.

It shows:

- the same request can be replayed without changing the economic outcome
- duplicate replay is cheap compared with the mixed burst path
- durable writes and audit publication are part of the end-to-end latency budget
- the burst tail is dominated by queueing and service contention, not by the numeric kernel alone

This is the part of the stack that determines whether the system is operationally trustworthy.

### Decision store / cache / audit stream

The benchmark exercises all three:

- PostgreSQL gives the terminal row and replay source
- Valkey suppresses duplicates on the hot path
- Redpanda receives the audit trail

The important result is that replay and duplicate handling remain deterministic when they are in the path.

## Contrast With v0

v0 is not comparable on this benchmark class.

It has no request envelope, no request identity, no durable terminal row, no replay log, no audit stream, and no EV gate.

| Property | v0 | v1 |
|---|---|---|
| Request model | local `Map[String, Double]` transforms | explicit request state machine |
| Durable terminal truth | none | PostgreSQL decision row |
| Replay | none | durable replay before recomputation |
| Audit | none | append-only audit trail |
| EV | none | estimate, lower bound, and reconciliation hooks |
| Concurrency model | Futures over a shared in-memory value | bounded orchestration with typed state transitions |
| Benchmarkability | not a service benchmark | measurable service benchmark |

The v0 demo can tell you whether a local function preserves balances.
It cannot tell you whether a bounded pre-trade service is viable.

## What Is Still Unproven

This benchmark does not prove realized execution outcome or economic viability under a live request population.

`EV_realized` requires:

- downstream execution
- post-trade reconciliation
- market data drift tracking
- slippage versus realized-fill comparison

It also does not prove production deployment viability at scale outside this local compose environment.
It does not show whether positive decision EV is common enough to cover operating cost with margin.
It does not show how often the gate should accept, defer, or reject in a real mix.
It does not show whether the model stays calibrated near the threshold.
It does not show whether the system survives repeated hot-key, stale-source, or duplicate-heavy traffic over time.

So the benchmark supports this conclusion, and only this conclusion:

- v1 is viable as a bounded pre-trade decision system.
- v1 is not yet proven as an end-to-end execution system.

## Bottom Line

The measured results support the design in `REALITY.md` and `tokenomics.md`.

The service:

- accepts only positive-lower-bound EV paths
- defers stale requests
- fails closed on oversized inputs
- preserves replay determinism
- keeps duplicate replay cheap
- maintains positive EV on accepted requests after uncertainty haircut

That is enough to say v1 is operationally plausible and economically disciplined at the decision layer.

It is not enough to claim realized execution profitability.
