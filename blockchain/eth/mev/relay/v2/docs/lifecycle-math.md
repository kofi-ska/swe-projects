# Lifecycle Math

Lower-bound model for v2. Derived from code. Excludes queueing delay, state/WAL/broker overhead, and transport effects.

## Symbols

- `Qcap` = `QUEUE_DEPTH` = 1024
- `W` = `WORKER_COUNT` = 4
- `R` = `MAX_RETRIES` = 3
- `B` = `RETRY_BACKOFF` = 500ms
- `Treq` = `REQUEST_TIMEOUT` = 2s
- `Qage` = `MAX_QUEUE_AGE` = 3s
- `Cin` = `MAX_INFLIGHT_PER_CLIENT` = 20
- `H` = `HISTORY_LIMIT` = 256

## Admission

For a bundle with `n` transactions:

```text
freshness = clamp((deadline - now) / Treq, 0, 1)
value     = VALUE_PER_TX * (1 + ln(1 + n)) * freshness
service   = backend-dependent service estimate in ms
cost      = service * COST_PER_MS + n * COST_PER_TX
net       = value - cost
priority  = net / service + freshness
accept    = slack > MIN_DEADLINE_SLACK
         && service <= slack
         && net >= MIN_NET_VALUE
```

The code rejects work when any check fails. This is an admission approximation, not a full lifecycle proof.

## Lifecycle

The bundle path is:

```text
received -> validated -> queued -> simulating -> simulated -> scored -> forwarded|rejected
```

Retry path:

```text
simulating -> retry_pending -> queued -> simulating
```

Terminal paths:

```text
forwarded -> persisted -> completed
rejected  -> persisted -> completed
dead_letter -> persisted -> completed
```

## Retry budget

Maximum simulation attempts:

```text
attempts_max = R
```

Retry delay after failed attempt `k`:

```text
delay(k) = k * B
```

The retry loop polls every `B/2`. That is scheduling granularity, not a guaranteed wait.

Worst-case elapsed time before terminal dead-letter, ignoring queue delay and persistence overhead:

```text
T_deadletter_max = R * Treq + sum(k * B for k=1..R) + B/2
```

With defaults:

```text
T_deadletter_max = 3 * 2s + (1+2+3) * 500ms + 250ms
                 = 9.25s
```

## Queue pressure

Steady-state service rate:

```text
mu_total = W / E[Tsim]
```

If simulation is timeout-bound:

```text
E[Tsim] <= Treq
mu_total ~= W / Treq = 4 / 2s = 2 bundles/s
```

So the conservative floor is about `2 bundles/s` per instance under timeout-bound load.

Worst-case drain time from a full queue:

```text
T_drain ~= Qcap / mu_total
```

At the timeout-bound floor:

```text
1024 / 2 = 512s
```

So a full queue takes about 8.5 minutes to drain if every worker hits the timeout boundary and no new work arrives. This is a single-instance floor.

## Stability

If arrival rate `lambda` is below service rate `mu_total`, queue depth stays bounded.

If `lambda > mu_total`, then:

```text
dQ/dt = lambda - mu_total
```

and the queue grows until it hits the unsafe boundary.

## Health thresholds

- degraded at 80% queue fill
- unsafe at full queue
- unsafe when stale work is present
- unsafe when `queue_age > Qage`
- draining means reject new work before shutdown

## What this means

- the code is bounded by design
- the conservative retry path is under 10 seconds before dead-letter, before queue and persistence overhead
- fixed buffer, known drain rate
- safe when arrival stays under service
- retry debt is a weighted pending-load proxy
- state hops reuse returned records instead of rereading after each transition
- WAL compaction is deferred until the log is well past the bound
