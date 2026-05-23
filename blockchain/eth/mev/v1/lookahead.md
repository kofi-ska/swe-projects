# v1 Lookahead

This file records work that is intentionally out of scope for v1.

## Deferred to v2

- Merkle trees for audit commitments and batch proofs
- stronger cryptographic verification paths
- proof-oriented record integrity
- deeper trust and verification boundaries
- distributed messaging if the runtime topology needs it

## Why it is deferred

- v1 already carries the relay lifecycle, bounded pressure handling, and hostile-load controls
- adding cryptographic commitment layers now would increase complexity without helping the first release prove the relay loop
- v2 is the right place to deepen verification once the v1 lifecycle is stable

## v1 rule

- keep the hot path simple
- use hashes, append-only records, and explicit state transitions
- defer proof systems until the relay behavior is stable
