# Recovery

This is the shard recovery path.

## Inputs

- snapshot
- WAL
- checkpoint index
- append-only events
- current lease / epoch / fence state

## Required checkpoint fields

- shard ID
- epoch
- event root
- event count
- last replayed offset
- checkpoint signature

If any of those are missing or do not match the replay result, the shard stays quarantined.

## Recovery order

1. load last valid snapshot
2. replay WAL
3. verify checkpoint root
4. compare event sequence and checkpoint state
5. quarantine on mismatch
6. acquire new lease
7. re-fence
8. rejoin only after validation

## Constraints

- replay does not write into live authority
- replay is idempotent with respect to checkpointed state
- ambiguous recovery stays quarantined
- checkpoints that do not match event history are rejected
- quarantine clears only after the shard has a fresh authority token

## Failure cases and outcomes

- corrupt checkpoint -> quarantine
- truncated WAL -> recover only if the boundary is safe and the checkpoint root still matches
- replay gap -> quarantine
- stale epoch -> reject rejoin
- mixed-version schema -> reject rejoin

## Operator checks

- current owner
- replay completeness
- checkpoint validity
- authority freshness
- queue age after rejoin
