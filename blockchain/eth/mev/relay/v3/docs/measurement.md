# Measurement

This file defines how the v3 lifecycle math is measured. It is the proof loop for the spec.
The runnable harness lives in `cmd/measure`.

## Goal

Measure the cost and stability of the shard lifecycle under steady load, burst, and fault injection.

The measurement loop answers:

- whether the queue stays stable
- whether authority stays fresh
- whether retries stay bounded
- whether recovery stays cold
- whether terminalization stays within budget
- whether observability distorts the system
- whether policy revisions move in the right direction
- whether recovery and rollout states stay bounded under faulted runs

## What is measured

Per shard `s`:

- `λ_in,s` incoming bundle rate
- `λ_dup,s` duplicate or replay rate
- `λ_retry,s` retry rate
- `E[Tsim,s]` simulation time
- `E[Tterm,s]` terminalization time
- `E[Twait,s]` queue wait time
- `E[Tauth,s]` authority check and renewal overhead
- `E[Trecover,s]` recovery time
- `δ` authority jitter
- queue age distribution
- queue depth distribution
- retry debt
- accept / reject rates
- shard skew
- failure-loss proxy
- tracing / metrics / logging overhead

## Measurement contract

Each run records:

- scenario name
- shard count
- worker count per shard
- queue depth
- backend kind
- payload size distribution
- retry cap
- lease TTL
- lease renew interval
- request timeout
- storage backend
- broker backend
- trace sampling rate
- log level
- warmup window
- measurement window
- cooldown window
- controller state before and after the run
- recovery / rollout state before and after the run

If one of those changes, the run is not comparable to the baseline.

## Baselines

Run against these baselines:

- idle shard
- low load
- steady-state load
- near-capacity load
- full queue
- backend slowdown
- broker slowdown
- state backend slowdown
- WAL pressure
- checkpoint pressure
- recovery path
- mixed-version rollout
- lease-jitter path

## Adversary model

The adversary is one or more of:

- duplicate submission flood
- replay flood
- retry amplification
- queue flooding
- backend slowness injection
- backend outage
- broker lag or outage
- state backend lag or outage
- checkpoint corruption
- lease renewal jitter
- stale authority reuse
- mixed-version overlap
- observability overload

The system is measured against that model, not against happy-path traffic.

## Experiment matrix

Each scenario varies one or more of:

- arrival rate
- duplicate rate
- retry rate
- backend latency
- state latency
- broker latency
- queue depth
- payload size
- shard count
- worker count
- lease jitter
- recovery state
- rollout state

Minimum scenarios:

1. steady-state throughput
2. burst load
3. burst plus duplicates
4. burst plus retries
5. backend slowdown
6. backend outage
7. broker outage
8. state outage
9. lease jitter
10. full recovery replay
11. mixed-version overlap
12. shard hotspot
13. observability pressure
14. queue saturation

## Report output

Each report records:

- scenario results
- health before and after
- metrics before and after
- latency percentiles
- regression flags against baseline
- recovery / rollout state at the end of the run

## Phase attribution

Every bundle and every background loop is timed by phase:

- ingress
- routing
- authority check
- admission
- queue push
- queue pop
- queue wait
- simulation
- state transition
- event append
- WAL append
- checkpoint seal
- retry claim
- retry requeue
- recovery replay
- re-fence
- rejoin
- rollout drain
- rollout cutover

Each phase reports:

- count
- sum
- histogram
- error count

Total latency without phase attribution is not sufficient.

## Statistics

For each metric, report:

- mean
- p50
- p95
- p99
- max
- variance
- sample count
- confidence interval

Use:

- mean for cost accounting
- p95 for normal operator planning
- p99 for tail behavior
- max for containment and failure review

Do not trust a single run.

## Instrumentation rules

- measure exporter overhead separately
- keep the same trace sampling rate inside a run
- keep label cardinality bounded
- do not let logs become the bottleneck
- record warmup and cooldown separately from the measurement window

## Authority timing

Measure:

- lease renewal latency
- renewal jitter
- scheduler delay
- network RTT
- clock skew
- store write latency

The authority condition is:

```text
τ > ρ + δ
```

Where:

- `τ` = lease TTL
- `ρ` = renew interval
- `δ` = renewal delay + scheduler delay + network jitter + clock skew

If the measured margin turns negative, the shard is unsafe.

## Queue math

Measure:

- queue depth
- queue age
- queue insertion cost
- queue pop cost
- queue eviction cost
- queue wait time

The stability condition is:

```text
λ_eff,s < μ_s
```

with:

```text
λ_eff,s = λ_in,s + λ_retry,s + λ_dup,s
μ_s = W_s / E[T_s]
```

If `λ_eff,s >= μ_s`, backlog grows.

## Retry math

Measure:

- retry count distribution
- retry success rate
- retry dead-letter rate
- retry wait time
- retry claim latency
- retry queue pressure

Retry debt is a weighted pending-load proxy:

```text
D_retry = Σ w(age_i, attempts_i, value_i)
```

The weight function must increase with age and attempts, and decrease with remaining value.

## Recovery math

Measure:

- snapshot load time
- WAL replay time
- checkpoint verify time
- quarantine decision time
- re-fence time
- rejoin time

Recovery is valid only if:

- replay does not write into live authority
- checkpoint root matches event history
- rejoin happens only after validation
- quarantine remains the sink on mismatch

## Terminalization math

Measure:

- state transition time
- event emit time
- WAL append time
- checkpoint seal time
- broker publish time
- cleanup time

Terminalization is a cost center. It is bounded, but not cheap.

## Observability overhead

Measure:

- metrics scrape latency
- trace export latency
- log volume
- label cardinality
- sampling overhead
- telemetry backpressure

Observability is part of lifecycle cost. It must be bounded.

## Acceptance criteria

A run is valid only if:

- the measurement contract is fixed
- the baseline is declared
- the adversary scenario is named
- the shard count is known
- the worker count is known
- the queue depth is known
- the backend kind is known
- the sample size is large enough
- the confidence interval is reported

A system state is acceptable only if:

- queue age stays within the measured safe band
- retry debt stays within the measured recovery band
- authority margin stays positive
- recovery completes within budget
- terminalization stays within budget
- observability overhead stays bounded
- mixed authority does not appear
- stale writes are rejected

## Measurement outputs

Each run should produce a table like this:

| Metric | Measured | Model | Delta |
|---|---:|---:|---:|
| `λ_in` |  |  |  |
| `λ_dup` |  |  |  |
| `λ_retry` |  |  |  |
| `E[Tsim]` |  |  |  |
| `E[Tterm]` |  |  |  |
| `E[Twait]` |  |  |  |
| `E[Tauth]` |  |  |  |
| `E[Trecover]` |  |  |  |
| queue age p99 |  |  |  |
| retry debt peak |  |  |  |
| authority margin |  |  |  |
| observability overhead |  |  |  |

## Decision rule

After each run:

1. compare measured values with the model
2. identify the dominant cost term
3. check whether the system stayed inside its bounds
4. update the constants if the model is wrong
5. keep or reject the implementation change based on the acceptance criteria

If the run violates the bounds, the run is a failure even if average throughput looks fine.

## Unknowns

The following values still need measurement in code:

- `λ_in,s`
- `λ_dup,s`
- `λ_retry,s`
- `E[Tsim,s]`
- `E[Tterm,s]`
- `E[Twait,s]`
- `E[Tauth,s]`
- `E[Trecover,s]`
- `δ`
- shard skew
- failure-loss term
- confidence threshold
