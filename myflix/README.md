# MyFlix

MyFlix is a greenfield, production-aware rewrite of the original video-streaming platform. The new system will use a Go modular-monolith backend, a SvelteKit web client, PostgreSQL as the source of truth, and object storage for direct media delivery.

Development is organised as a maturity journey:

`v0`: prove the core user journey with a small, usable system.

`v1`: establish production-shaped boundaries, bounded concurrency, observability, recovery, and MinIO-backed media delivery.

`v2`: promote the same architecture onto managed GCP infrastructure, GCS, and Cloud CDN.

`v3`: mature the product around economics, governance, compliance, experimentation, and regional scale.

Each version should validate its decisions with tests and operational evidence. Business rules remain in the backend, PostgreSQL remains authoritative, and the Go API authorises media access without proxying production video traffic.
