# Failure Taxonomy

## Retryable

- transient backend failures
- transient broker failures
- temporary state pressure when still safe

## Terminal

- malformed request
- duplicate bundle
- queue overflow
- deadline too old
- economically negative work
- retry budget exhausted

## Unsafe

- WAL failure
- state backend failure
- broker failure when fan-out is safety-critical
- checkpoint corruption
- replay inconsistency
- queue full
- stale work present

## Operator meaning

- retryable: try again if value remains positive
- terminal: stop the bundle
- unsafe: stop or drain the relay
