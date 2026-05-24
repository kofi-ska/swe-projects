# Lifecycle Math

Lower-bound model for v3. The queue, retry, and drain math come from v2. v3 adds lease renewal and re-fencing overhead.

## Symbols

- `Qcap` = `QUEUE_DEPTH` = 1024
- `W` = `WORKER_COUNT` = 4
- `R` = `MAX_RETRIES` = 3
- `B` = `RETRY_BACKOFF` = 500ms
- `Treq` = `REQUEST_TIMEOUT` = 2s
- `Qage` = target queue age = 1s
- `Cin` = `MAX_INFLIGHT_PER_CLIENT` = 20
- `H` = `HISTORY_LIMIT` = 256

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

The code rejects work when any check fails. This is a lower-bound admission model, not a proof of final outcome.

## Lifecycle

```text
received -> validated -> queued -> simulating -> simulated -> scored -> forwarded|rejected
```

Retry path:

```text
simulating -> retry_pending -> queued -> simulating
```

Terminal path:

```text
forwarded -> persisted -> completed
rejected  -> persisted -> completed
dead_letter -> persisted -> completed
```

## Retry budget

```text
attempts_max = R
delay(k) = k * B
```

Retry loop poll interval:

```text
B / 2
```

Worst-case elapsed time before dead-letter, ignoring queue delay and persistence overhead:

```text
T_deadletter_max = R * Treq + sum(k * B for k=1..R) + B/2
```

With defaults:

```text
T_deadletter_max = 3 * 2s + (1+2+3) * 500ms + 250ms
                 = 9.25s
```

This is the lower bound. Lease renewal, re-fencing, queue delay, and WAL work add more time.

## Queue pressure

```text
mu_total = W / E[Tsim]
```

If simulation is timeout-bound:

```text
mu_total ~= W / Treq = 4 / 2s = 2 bundles/s
```

Drain time from a full queue:

```text
T_drain ~= Qcap / mu_total
```

At the timeout-bound floor:

```text
1024 / 2 = 512s
```

This is a single-shard floor. Shard-local authority adds overhead and must be measured separately.

## Stability

If `lambda < mu_total`, queue depth stays bounded.

If `lambda > mu_total`, then:

```text
dQ/dt = lambda - mu_total
```

and the queue grows until it hits the unsafe boundary.

## Thresholds

- degraded at 80% queue fill
- unsafe at full queue
- unsafe when stale work is present
- unsafe when queue age exceeds target
- draining rejects new work before shutdown

## Notes

- retry debt is a weighted pending-load proxy
- state transitions reuse returned records instead of rereading after each hop
- WAL compaction stays off the common path until the log is well past the bound
