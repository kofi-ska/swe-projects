# Alerting

Alerting covers unsafe, degraded, uneconomic, or blind states.

## Page now

- unsafe health
- queue full
- queue age beyond bound
- stale work present
- backend unavailable
- state backend unavailable
- broker unavailable
- WAL unavailable
- checkpoint failure
- replay inconsistency
- duplicate spike
- metrics silence

## Warn

- queue pressure above 80%
- retry debt, interpreted as weighted pending load rather than exact cost
- low queue value
- backend latency rise
- state latency rise
- broker latency rise
- dead-letter rise
- inflight saturation

## Delivery

- external alert manager or cloud monitoring
- app emits signals only
- every alert links to a runbook
- [`alerting-rules.yaml`](./alerting-rules.yaml) is the starting policy set
- local compose validates alert wiring only

## Escalation

- unsafe: stop or drain
- degraded: watch and prepare
- warn: investigate

## Thresholds

- queue fill warning: 80%
- queue full: 100%
- queue age warning: 1.5s
- queue age unsafe: 3s
- retry budget: 3 attempts
