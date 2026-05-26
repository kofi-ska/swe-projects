# Observability

## Signals

### Counters

- submitted bundles
- accepted bundles
- rejected bundles
- forwarded bundles
- dead letters
- retry scheduling
- duplicates
- queue overflow
- inflight limit hits
- backend errors
- state errors
- WAL errors
- broker errors
- terminal errors

### Gauges

- queue depth
- queue capacity
- queue oldest age
- queue net value
- retry debt, as a weighted pending-load proxy
- health state
- backend latency
- state latency
- broker latency
- WAL latency

## Surfaces

- `/healthz`: posture
- `/readyz`: admission safety
- `/metrics`: alerts and dashboards

## Rules

- trace lifecycle transitions
- log stable IDs only
- no raw payloads in logs
- keep cardinality bounded
- alert on state, not process liveness
- queue net value and retry debt are proxies
