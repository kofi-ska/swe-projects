# Recovery

Deterministic recovery. Bounded replay. Rejoin only after validation.

## Procedure

1. validate the WAL is readable
2. validate state backend health
3. load latest bundles and checkpoints
4. replay bounded history
5. verify checkpoint roots and event consistency
6. verify stale terminal state cannot overwrite live truth
7. re-enable readiness only after checks pass

## Stop conditions

- checkpoint mismatch
- WAL corruption
- replay divergence
- state/backend mismatch
- duplicate terminalization

## Rejoin rule

No shard or instance rejoins while replay is uncertain or evidence is inconsistent.
