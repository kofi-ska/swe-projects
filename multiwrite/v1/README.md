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
- Bound records, bytes, duration, record size, and buffer size.
- Reject queue and batch controls until those paths are implemented.
- Generate deterministic workloads from seed and profile.
- Include flush, close, and commit in lifecycle timing.
- Write through a temp artifact and commit deliberately.
- Emit a versioned JSON manifest for every terminal run.
- Record terminal reason separately from status.
- Preserve enough environment metadata to reproduce and compare local, Docker Compose, and k3s runs.
- Expose profiling artifacts through CLI flags.

## Docs

- [Architecture](./docs/Architecture.md): run contract, entities, config, lifecycle, data flow, formal model, queueing model, manifest schema, interfaces, and deployment requirements.
- [Benchmarks](./docs/Benchmarks.md): benchmark protocol and result records for local, Docker Compose, and k3s.
- [Benchmark analysis](./benchmarks/analysis.md): current measured profiling results, pprof findings, trace findings, benchmark results, and remaining evidence gaps.

## Layout

- `src/`: importable write engine package.
- `src/internal/multiwrite/`: engine implementation.
- `src/cmd/multiwrite/`: CLI entrypoint.
- `tests/`: external tests and benchmarks that import `src/`.
- `benchmarks/`: benchmark configs, results, and generated artifacts.

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

## Run Locally

From `multiwrite/v1`:

```bash
go run ./src/cmd/multiwrite run \
  --writer=buffered \
  --profile=fixed \
  --records=100000 \
  --max-bytes=67108864 \
  --max-duration=30s \
  --record-size=32 \
  --buffer-size=16384 \
  --seed=101 \
  --out=benchmarks/artifacts/outputs/local-buffered-fixed.txt \
  --manifest=benchmarks/artifacts/manifests/local-buffered-fixed.json \
  --durability=closed \
  --overwrite=replace
```

Run package benchmarks:

```bash
go test ./... -bench=. -benchmem
```

Writer microbenchmarks isolate direct vs buffered writing with prebuilt payloads. Engine lifecycle benchmarks call the full `Run(config)` path.

Run the bounded Compose smoke case:

```bash
docker compose run --rm multiwrite-smoke
```

Run the Compose direct and buffered comparison services:

```bash
docker compose --profile benchmark up --build --abort-on-container-exit
```

Compose parameters are defined in `.env`.
