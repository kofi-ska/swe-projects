# multiwrite v1

`multiwrite` v1 is a bounded, reproducible write workload engine.

It replaces the v0 loop-only measurement with a manifest-driven run model that records config, seed, workload profile, environment, artifact state, timings, and failure domain.

## v0 Gaps

- Timing excludes final flush and close.
- Close errors are ignored.
- Output reports records and elapsed time only.
- Inputs are not validated as a boundary contract.
- Records, bytes, duration, memory, queue depth, and disk use are not bounded.
- Random generation is not explicitly seeded.
- Output is written directly to the final path.
- Partial output is not classified.
- No manifest records config, seed, environment, artifact state, or failure domain.

## v1 Contract

- Validate config before file creation or generation.
- Bound records, bytes, duration, queue depth, batch size, and buffer size.
- Generate deterministic workloads from seed and profile.
- Include flush, close, and commit in lifecycle timing.
- Write through a temp artifact and commit deliberately.
- Emit a versioned JSON manifest for every terminal run.
- Preserve enough environment metadata to reproduce and compare local, Docker Compose, and k3s runs.
- Expose profiling artifacts through CLI flags.

## Docs

- [Architecture](./docs/Architecture.md): run contract, entities, config, lifecycle, data flow, formal model, queueing model, manifest schema, interfaces, and deployment requirements.
- [Benchmarks](./docs/Benchmarks.md): benchmark protocol and result records for local, Docker Compose, and k3s.

## CLI Shape

```bash
multiwrite run \
  --records <record-count> \
  --max-bytes <byte-limit> \
  --max-duration <duration-limit> \
  --profile <profile> \
  --seed <seed> \
  --out <output-path> \
  --manifest <manifest-path> \
  --durability <closed|synced|committed> \
  --overwrite <fail|replace|version>
```
