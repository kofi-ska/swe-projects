# Security

## Minimum posture

- validate at ingress
- do not trust caller identity from transport metadata alone
- keep secrets out of logs and traces
- redaction is mandatory for payload-adjacent data
- treat broker messages as transport, not authority
- keep state and event data behind least-privilege boundaries
- `API_AUTH_TOKEN` can gate submit requests without changing the public API contract

## Abuse control

- max payload size
- bounded retries
- bounded inflight per client
- duplicate detection
- fail closed on malformed or unsafe input

## Notes

Minimum safe operating posture. Not a full security program.

Not included:

- rate limiting
- mTLS or mutual auth
- per-client authorization policy
- response-body redaction for all failure paths
- WAF or abuse-detection integration
