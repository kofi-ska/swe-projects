# Testing

## Required coverage

- request validation
- duplicate handling
- queue overflow
- stale work rejection
- retry scheduling
- terminal state transitions
- checkpoint creation
- health state transitions
- metrics export
- backend failure handling
- WAL failure handling
- broker failure handling
- state corruption handling
- auth-required submission when configured
- draining readiness behavior
- `/metrics` availability

## Nice-to-have coverage

- load tests
- soak tests
- race tests
- restart tests
- replay tests
- malformed request fuzzing

## Rule

Do not merge lifecycle, retry, or persistence changes without a test for the new path.
