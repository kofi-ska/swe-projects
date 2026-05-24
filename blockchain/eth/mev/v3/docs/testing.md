# Testing

This is the current test matrix implied by the v3 model.

## Coverage

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

## Concurrency tests

- concurrent writers on one shard
- lease expiry during in-flight work
- retry and terminalization races
- queue push/pop races
- shutdown while work is active

## Failure tests

- backend timeout
- backend outage
- broker outage
- state outage
- WAL failure
- checkpoint corruption
- stale chain view
- recovery mismatch

## Load tests

- queue age under burst
- retry debt under burst
- terminalization cost under burst
- drain time under full queue
- shard hotspot behavior

## Regression tests

- public response codes
- ready / unsafe / draining transitions
- idempotent duplicate handling
- alert rule thresholds
- docs-backed operating envelope
