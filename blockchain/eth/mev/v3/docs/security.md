# Security

This is the current trust boundary and the minimum hardening described in the repo.

## Trust boundary

- public HTTP ingress is the boundary
- everything past ingress is controlled input
- transport metadata is not authority

## Minimum controls in the current design

- request size limits
- explicit auth where required
- least privilege for state, broker, and storage
- secret storage outside source control
- payload redaction in logs and traces
- replay-abuse resistance through dedupe and fencing

## Rejected inputs

- malformed payloads
- oversized payloads
- duplicate submissions that violate idempotency
- stale authority writes
- unsafe backend input

## Log exclusions

- raw bundle payloads
- secrets
- high-cardinality user data
- opaque blobs that do not help incident response

## Not covered yet

- full authN/authZ design
- rate limiting policy
- hardening for public exposure beyond the current baseline
