# Testing

This is the test matrix implied by the v3 model.

## Required coverage

- validation
- admission
- queueing
- retry deadlines
- terminalization
- checkpoint sealing
- recovery replay
- authority fencing
- stale write rejection
- health transitions
- metrics rendering
- alert rule evaluation

## Required suites

### Concurrency

- concurrent writers on one shard
- lease expiry during in-flight work
- retry and terminalization races
- queue push/pop races
- shutdown while work is active

### Failure

- backend timeout
- backend outage
- broker outage
- state outage
- WAL failure
- checkpoint corruption
- stale chain view
- recovery mismatch

### Load

- queue age under burst
- retry debt under burst
- terminalization cost under burst
- drain time under full queue
- shard hotspot behavior

### Regression

- public response codes
- ready / unsafe / draining transitions
- idempotent duplicate handling
- alert rule thresholds
- docs-backed operating envelope

## Merge gate

A change that touches the hot path should not merge unless:

- the affected unit tests pass
- the affected concurrency tests pass
- the relevant failure tests pass
- the relevant load test is run or updated
- the public response codes are unchanged or intentionally versioned
- the operating bounds in the docs still match the code

## Release gate

Before release:

- recovery replay is tested
- fence rejection is tested
- stale authority is tested
- queue saturation is tested
- observability output is rendered
- alert rules are evaluated
