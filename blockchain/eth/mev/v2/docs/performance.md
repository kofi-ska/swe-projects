# Performance

## Current limits

| Limit | Value |
|---|---:|
| Queue depth | 1024 |
| Worker count | 4 |
| Retry count | 3 |
| Retry backoff | 500ms |
| Request timeout | 2s |
| Max queue age | 3s |
| Max in-flight per client | 20 |
| History limit | 256 |
| WAL max entries | 2048 |

## Derived budgets

- worst-case retry path: 9.25s before dead-letter, before queue and persistence overhead
- timeout-bound throughput floor: 2 bundles/s per instance
- full queue drain: 512s at that floor
- queue age > 3s: unsafe
- queue fill at 80%: degraded
- retry debt: weighted pending-load proxy
- state transitions reuse returned records; extra reloads are avoided on the hot path
- WAL compaction is not on every append after the limit; it waits until the log is well past the bound

## Expectations

- hot path stays allocation-light
- rejection is cheaper than acceptance
- queue age is visible and acted on
- p99 matters more than average latency
- the math is conservative and lower-bound
- event and checkpoint payloads are encoded once per write path

## Operational rule

If a workload no longer fits the budget, shed it early.
