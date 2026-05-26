# v3 Operations

This file describes the current operational state model, signals, alert classes, and response patterns.

## Signals in the system

- request rate
- accept rate
- reject rate
- queue depth
- queue age
- queue net value
- retry debt
- worker saturation
- backend latency
- state latency
- broker latency
- terminalization latency
- checkpoint latency
- dead-letter rate
- authority state
- recovery state

## States reported by health

| State | Meaning |
|---|---|
| Ready | authority current, queue age under target, confidence above threshold |
| Degraded | pressure rising but still bounded |
| Unsafe | stale authority, queue full, low confidence, or recovery inconsistency |
| Draining | stop new work, finish in-flight work, seal state, release authority |

## Alert classes

### Page now

- stale authority
- split-brain suspected
- recovery inconsistency
- checkpoint corruption
- WAL corruption
- queue full
- unsafe health
- backend unavailable
- state backend unavailable
- broker unavailable
- telemetry silence

### Investigate

- queue age above target
- retry debt rising
- dead-letter rate rising
- backend latency rising
- state latency rising
- broker latency rising
- hot shard detected
- rollout drift
- config drift

### Track

- acceptance rate change
- throughput drop
- payload size drift
- per-client inflight skew
- confidence drift

## Alert to action

| Alert class | Action |
|---|---|
| Page now | stop-accepting, draining, or quarantine |
| Investigate | verify the affected path and compare with the last good run |
| Track | file a ticket and review later |

## First responses

### Authority loss

- stop accepting new work
- drain in-flight work
- confirm current owner
- re-fence or hand off

### Queue saturation

- reject new work
- shed low-value work
- check worker and backend throughput
- restore queue age before reopen

### Retry storm

- cap retries
- check backend health
- confirm expected value stays positive
- dead-letter stale work

### Recovery failure

- quarantine the shard
- restore the last valid checkpoint
- rerun replay validation
- re-fence before rejoin

### Mixed-version rollout

- drain the new owner
- revert to the last safe version
- keep the shard fenced until one owner remains

### Chain confidence failure

- fail closed
- reject new work
- confirm backend freshness
- resume only after confidence recovers

### Observability failure

- treat it as degraded
- confirm scrape and export paths
- fix telemetry before trusting health

## Failure classes seen by operators

- client error: malformed request, payload too large, unauthorized, duplicate, stale precondition, economically invalid
- capacity error: queue full, inflight limit exceeded, budget exceeded, queue age over target
- dependency error: backend timeout, backend unavailable, broker unavailable, state backend unavailable, WAL failure
- safety error: stale authority, recovery inconsistency, split-brain suspected, low chain confidence, audit divergence
- internal error: invariant failure, panic

## Log rules

- no raw payloads by default
- no secrets
- no unbounded labels
- one stable ID per request and bundle
- keep labels bounded

## Accessibility for operators

- use short state names
- keep errors specific
- keep IDs copyable
- do not rely on color alone
- keep `/healthz` and `/readyz` readable in a terminal
- use the same state labels in docs, logs, metrics, and alerts

## Operator checks

- `/healthz` shows current state
- `/readyz` shows acceptability for new work
- `/metrics` exposes the scrape surface
- traces must identify shard and authority
