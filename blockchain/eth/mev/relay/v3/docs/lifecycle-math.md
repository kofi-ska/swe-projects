# Lifecycle Math

This is the lower-bound lifecycle model. Queueing, retry, and drain math come from the current v3 defaults. Lease renewal and re-fencing add extra cost on top.

## Symbols

| Symbol | Value | Meaning |
|---|---:|---|
| `Qcap` | `1024` | queue depth per shard |
| `W` | `4` | workers per shard |
| `R` | `3` | retry cap |
| `B` | `500ms` | retry backoff |
| `Treq` | `2s` | request timeout |
| `Qage` | `1s` | target queue age |
| `Cin` | `20` | max inflight per client |
| `H` | `256` | retained history |

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

The constants are policy values. The runtime must set them from config or rollout parameters, then verify them against measurement.

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
poll_interval = B / 2
```

Worst-case elapsed time before dead-letter, ignoring queue delay and persistence overhead:

```text
T_deadletter_max = R * Treq + sum(k * B for k=1..R) + B/2
```

With defaults:

```text
T_deadletter_max = 3 * 2s + (1 + 2 + 3) * 500ms + 250ms
                 = 9.25s
```

This is a lower bound. Queue delay, lease renewal, and WAL work add more time.

## Queue pressure

```text
μ_total = W / E[Tsim]
```

If simulation is timeout-bound:

```text
μ_total ~= W / Treq = 4 / 2s = 2 bundles/s
```

Drain time from a full queue:

```text
T_drain ~= Qcap / μ_total
```

At the timeout-bound floor:

```text
1024 / 2 = 512s
```

This is a single-shard floor. Shard authority adds overhead and must be measured.

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
