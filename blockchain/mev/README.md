# MEV

Trusted, low-latency decision layer for private MEV flow.

This workspace holds the Go implementation of the relay stack. We start with Anvil and a modular monolith, then harden the same shape into production later.

## What We Are Building

- public bundle intake
- relay core for validation, queueing, and coordination
- simulator for local or forked EVM execution
- scoring and decisioning
- builder handoff or mock publish
- storage, audit, and observability

## Architecture

- modular monolith in Go
- domain modules for `searcher`, `relay`, `builder`, and `validator`
- swappable chain backend
- Anvil by default
- gRPC services only if a boundary proves worth splitting out

## Versions

### v1
First release.

- submit bundle
- validate request
- queue work
- simulate on Anvil
- score result
- persist outcome
- audit and observe

### v2
Second release.

- auth
- request signing
- rate limiting
- idempotency
- bounded queues and backpressure
- metrics, logs, and alerts
- configurable backend

### v3
Third release.

- stronger security
- replayability
- backup and restore
- load and soak testing
- incident playbooks
- privacy controls

### v4
Fourth release.

- customer-specific policy
- support and escalation
- pricing and unit economics
- retention
- compliance

## Docs

- [v1 README](v1/README.md)
