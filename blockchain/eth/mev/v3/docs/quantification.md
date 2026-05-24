# v3 Quantification

This file turns the v3 spec into equations and bounds. It is the constraint sheet for the system.

## System model

For shard `s`:

```text
λ_eff,s = λ_in,s + λ_retry,s + λ_dup,s
μ_s     = W_s / E[T_s]
stable  iff λ_eff,s < μ_s
```

Where:

- `λ_in,s` = incoming bundle rate
- `λ_retry,s` = retry rate
- `λ_dup,s` = duplicate or resend rate
- `W_s` = worker count per shard
- `E[T_s]` = mean service time per accepted bundle

Queue growth:

```text
dQ_s/dt = λ_eff,s - μ_s
```

Queue is bounded only when `λ_eff,s < μ_s`.

## Admission

For a bundle with `n` transactions:

```text
freshness = clamp((deadline - now) / Treq, 0, 1)
value     = VALUE_PER_TX * (1 + ln(1 + n)) * freshness
service   = backend estimate in ms
cost      = service * COST_PER_MS + n * COST_PER_TX
net       = value - cost
priority  = net / service + freshness
accept    = slack > MIN_DEADLINE_SLACK
         && service <= slack
         && net >= MIN_NET_VALUE
```

Admission is value-gated. The runtime accepts work only when expected net value is positive under current authority and chain confidence.

## Retry

Defaults:

```text
R = 3
B = 500ms
poll = B / 2 = 250ms
```

Worst-case lower bound to dead-letter, ignoring queue delay and persistence overhead:

```text
T_deadletter_max = R * Treq + sum(k * B for k=1..R) + B/2
```

With `Treq = 2s`:

```text
T_deadletter_max = 3 * 2s + (1 + 2 + 3) * 500ms + 250ms
                 = 9.25s
```

Retry debt is a weighted pending-load proxy:

```text
D_retry = Σ w(age_i, attempts_i, value_i)
```

The weight function must increase with age and attempts, and decrease with remaining value.

## Queue pressure

Defaults:

```text
Qcap = 1024
W    = 4
Treq = 2s
```

Timeout-bound service floor:

```text
μ_s ~= W / Treq = 4 / 2s = 2 bundles/s
```

Drain floor for a full shard:

```text
T_drain ~= Qcap / μ_s = 1024 / 2 = 512s
```

Threshold bands:

```text
healthy  : queue_age < 1s
warn     : 1s <= queue_age < 12s
unsafe   : queue_age >= 12s
```

## Authority

Defaults:

```text
τ = 5s
ρ = 1s
```

Safe authority margin:

```text
SafeAuthorityMargin = τ - ρ - δ
```

Where `δ` is renewal delay, scheduling delay, network jitter, and clock skew.

Authority is safe only if:

```text
τ > ρ + δ
```

## Recovery

Retained history:

```text
H = 256
WAL_max = 2048
```

Recovery work is bounded by:

```text
O(H + WAL_max)
```

Replay order:

```text
snapshot -> WAL -> checkpoint -> re-fence -> rejoin
```

Recovery is valid only if replay does not write into live authority and rejoin happens after validation.

## Security bounds

Defaults:

```text
MAX_PAYLOAD_BYTES = 256KiB
MAX_INFLIGHT_PER_CLIENT = 20
QUEUE_DEPTH = 1024
```

Upper envelope per client:

```text
20 * 256KiB = 5MiB
```

Upper queued payload envelope per shard:

```text
1024 * 256KiB = 256MiB
```

These are upper bounds, not measured heap footprint.

## Observability

The system exports bounded signals for:

- request rate
- accept / reject rate
- queue depth / age / value
- retry debt
- worker saturation
- backend / state / broker latency
- terminalization latency
- checkpoint latency
- dead-letter rate
- authority state
- recovery state

Total series count must stay bounded by label cardinality:

```text
series_total = Σ L_i
```

where `L_i` is the cardinality of metric `i`.

## Economics

Operating cost is negative by default until value preservation or revenue is proven.

```text
ExpectedNet = ExpectedValue - DelayCost - ComputeCost - RetryCost - FailureRisk
```

Admission requires:

```text
ExpectedNet > 0
```

Cost per accepted bundle:

```text
C_bundle = C_month / A
```

Where:

- `C_month` = monthly operating cost
- `A` = accepted bundles per month

## Graph constraints

Authority dominates mutation:

```text
∀ path ingress -> mutation, authority ∈ path
```

Retries are layered:

```text
layer 0 -> layer 1 -> layer 2 -> layer 3 -> sink
```

Recovery is constrained reachability:

```text
snapshot -> WAL -> checkpoint -> re-fence -> rejoin
```

Rollout is a partial order:

```text
drain -> cutover -> rollback
```

## Launch gate

Promotion requires all of:

- public compatibility
- one owner per shard
- stale-writer rejection
- re-fence before rejoin
- bounded queue age
- bounded retry debt
- fail-closed confidence
- reconstructable audit truth
- sampled observability
- cost inside envelope
- cutover without mixed authority

## Unknowns that still need measurement

- `λ_in,s`
- `λ_dup,s`
- `λ_retry,s`
- `E[T_s]`
- `δ`
- `E[Tsim]`
- failure loss term
- shard count and shard skew
- confidence threshold
