# Runbooks

## 1. Queue pressure

- inspect queue depth, age, and net value
- stop accepting new work if unsafe
- check backend and state latency
- reduce upstream load

## 2. Retry storm

- inspect retry debt and dead-letter rate
- check backend stability
- verify retry budget is bounded
- drain stale work

## 3. Backend outage

- mark unsafe
- stop acceptance
- verify adapter health
- restore backend before readiness

## 4. State or WAL failure

- stop acceptance
- verify append-only evidence and state backend health
- recover from last known good state
- do not rejoin until replay is consistent

## 5. Broker failure

- keep acceptance closed if fan-out is safety-critical
- verify publish latency and subscriber health
- restart broker or switch transport path

## 6. Duplicate or malformed traffic

- verify request validation and duplicate detection
- keep rejecting cheap junk at ingress
- inspect inflight and payload size patterns

## 7. Observability outage

- verify `/metrics`, `/healthz`, `/readyz`
- restore metric export first
- do not trust silent nodes
