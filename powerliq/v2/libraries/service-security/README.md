# Service security

Production deployments must activate the `prod` Spring profile and provide `POWERLIQ_JWT_ISSUER_URI`.
The shared resource-server policy validates JWT signature, issuer and expiry, uses no HTTP session, and maps scopes as follows:

- `powerliq/read`: `GET /api/**`
- `powerliq/write`: mutations under `/api/**`
- `powerliq/internal`: `/internal/**` event delivery
- unauthenticated: liveness/readiness health only

Local development explicitly sets `powerliq.security.enabled=false`. Production configuration is fail-closed.
