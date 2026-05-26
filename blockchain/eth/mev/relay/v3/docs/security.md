# Security

This is the trust boundary and minimum hardening for the current v3 model.

## Trust boundary

- public HTTP ingress is the boundary
- everything past ingress is controlled input
- transport metadata is not authority

## Minimum controls

- request size limits
- explicit auth where required
- least privilege for state, broker, and storage
- secret storage outside source control
- payload redaction in logs and traces
- replay-abuse resistance through dedupe and fencing

## Request rules

- reject malformed payloads
- reject oversized payloads
- reject duplicate submissions that violate idempotency
- reject stale authority writes
- reject unsafe backend input

## Log rules

- no raw bundle payloads
- no secrets
- no high-cardinality user data
- no opaque blobs that do not help incident response
- metrics should carry IDs, not payloads

## Not covered yet

- full authN/authZ design
- rate limiting policy
- hardening for public exposure beyond the current baseline
