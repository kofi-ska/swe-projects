# Migrations (v1)

## Instance Pinning

Workflow instances are pinned to the `specId` and `specVersion` they were created with.
The runtime rejects execution if the engine is configured with a different `specId/specVersion`.

Rationale:

- Avoids silent semantic drift for in-flight workflows.
- Forces explicit, auditable upgrades.

## Upgrade Strategy (Recommended)

1. Publish a new spec version (e.g. `specVersion = 2`) alongside v1.
2. Stop routing new instances to v1 (new instances start on v2).
3. Migrate in-flight instances explicitly:
   - Load instance state/context/history
   - Apply a migration function: `(v1 instance) -> (v2 instance)`
   - Write back with optimistic concurrency
4. Resume processing on v2.

## v1 Non-Goals

- Automatic, implicit migrations.
- Running an instance on multiple spec versions.

