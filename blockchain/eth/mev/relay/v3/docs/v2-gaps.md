# v2 Gaps and v3 Response

v2 was a bounded relay with durable state, retry limits, health, and recovery. That is the base.

## Gaps in v2

- no shard authority model
- no separate policy controller
- no persisted policy state
- no shard-specific adaptation
- no recovery controller as its own subsystem
- no rollout controller as its own subsystem
- no proof loop against the runtime
- no long-horizon feedback loop

## v3 response

- shard authority is lease / epoch / fence
- policy adapts from measured pressure and confidence
- policy snapshots persist in Valkey
- recovery and rollout each have their own controller state
- `cmd/measure` runs steady, burst, and faulted scenarios
- the same runtime stack is used for the relay path and the proof loop

## What we are building

An adaptive multi-layer control system that keeps invariant-preserving state transitions bounded under uncertainty, partial observability, and competing objectives.

The claim is earned when the runtime, the measurement loop, and the deployment path all agree.
