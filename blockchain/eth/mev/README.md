# MEV Relay

Ethereum / EVM MEV relay workspace.

This project is organized as a versioned strategy:

- `v0` legacy prototype
- `v1` bounded hostile relay
- `v2` auditable, replayable, brokered relay
- `v3` distributed blockchain rigor

## Strategy

### v0
Prototype shape only.

- relay / simulator / builder split
- Compose deployment
- gRPC between services
- TimescaleDB and Superset
- Anvil-backed simulation

### v1
Bounded control.

- accept
- validate
- queue
- simulate
- score
- persist
- decide

Focus:
- lifecycle state machine
- bounded queueing
- bounded retries
- hostile-load handling
- deterministic failure modes
- audit-friendly records

### v2
Audit and evidence.

- append-only events
- batch commitments
- signed checkpoints
- replay-safe dedupe
- brokered work distribution
- OTEL traces / metrics / logs
- recovery and retention controls

Focus:
- tamper-evident history
- replayability
- throughput with evidence
- audit-readiness

### v3
Distributed rigor.

- consistency semantics
- partition tolerance
- multi-region behavior
- rollout and migration
- fairness and scheduling
- storage corruption recovery
- economic and adversarial modeling

Focus:
- mainnet-grade distributed behavior
- consensus / ownership / recovery rigor
- blockchain-system correctness

## Architecture

- Go modular monolith first
- domain modules for `searcher`, `relay`, `builder`, and `validator`
- Anvil by default for local execution
- chain backend stays swappable
- gRPC only if a boundary earns it

## Current docs

- [v1 README](v1/README.md)
- [v1 Lookahead](v1/lookahead.md)
- [v2 README](v2/README.md)
- [v3 README](v3/README.md)
