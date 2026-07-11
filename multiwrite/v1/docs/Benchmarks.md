# Benchmarks

## Protocol

Benchmark results are accepted only with a manifest.

Each result records:

- Command.
- Manifest path.
- Environment class.
- Resource envelope.
- Workload profile.
- Seed.
- Durability mode.
- Artifact state.
- Total lifecycle duration.
- Flush and close duration.
- Records written.
- Bytes written.
- Records per second.
- MiB per second.
- Error domain.
- Profile artifact paths.

## Required Manifest Fields

- `schema_version`
- `run_id`
- `binary_version` or git commit
- `started_at`
- `ended_at`
- `status`
- `error_domain`
- `records_requested`
- `records_written`
- `bytes_written`
- `duration_total_ms`
- `duration_flush_close_ms`
- `records_per_second`
- `mib_per_second`
- `seed`
- `workload_profile`
- `durability`
- `buffer_size`
- `batch_size`
- `queue_depth`
- `environment.class`
- `environment.cpu_limit`
- `environment.memory_limit`
- `environment.filesystem`
- `environment.volume_kind`

k3s results also record:

- `environment.storage_class`
- `environment.pvc_size_bytes`
- `environment.node_name`
- `environment.pod_name`
- `environment.container_image`

## Workload Matrix

- `fixed`
- `tiny`
- `wide`
- `heavy-tail`
- `high-entropy`
- `low-entropy`
- `burst`

## Durability Matrix

- `closed`
- `synced`
- `committed`

## Environment Matrix

- Local
- Docker Compose
- k3s

## Local Run Record

Command:

```bash
multiwrite run \
  --records <record-count> \
  --max-bytes <byte-limit> \
  --max-duration <duration-limit> \
  --profile <profile> \
  --seed <seed> \
  --out ./artifacts/output/local.txt \
  --manifest ./artifacts/manifests/local.json \
  --durability <closed|synced|committed> \
  --overwrite <fail|replace|version>
```

Record:

- Manifest:
- Status:
- Error domain:
- Records written:
- Bytes written:
- Total duration:
- Flush/close duration:
- Records/s:
- MiB/s:
- CPU profile:
- Heap profile:
- Trace:

## Docker Compose Run Record

Command:

```bash
docker compose run --rm multiwrite run \
  --records <record-count> \
  --max-bytes <byte-limit> \
  --max-duration <duration-limit> \
  --profile <profile> \
  --seed <seed> \
  --out /artifacts/output/compose.txt \
  --manifest /artifacts/manifests/compose.json \
  --durability <closed|synced|committed> \
  --overwrite <fail|replace|version>
```

Record:

- Manifest:
- Status:
- Error domain:
- Records written:
- Bytes written:
- Total duration:
- Flush/close duration:
- Records/s:
- MiB/s:
- CPU limit:
- Memory limit:
- Volume kind:
- Container image:

## k3s Run Record

Command:

```bash
kubectl apply -f ./deploy/k3s/job.yaml
kubectl logs -f job/multiwrite
kubectl cp <pod-name>:/artifacts/manifests/k3s.json ./artifacts/manifests/k3s.json
```

Record:

- Manifest:
- Status:
- Error domain:
- Records written:
- Bytes written:
- Total duration:
- Flush/close duration:
- Records/s:
- MiB/s:
- CPU request/limit:
- Memory request/limit:
- Storage class:
- PVC size:
- Node:
- Pod:
- Container image:

## Comparison Rules

A comparison is valid when these manifest fields match:

- Binary version or git commit.
- Workload profile.
- Seed.
- Record limit.
- Byte limit.
- Durability mode.
- Buffer size.
- Batch size.
- Queue depth.
- Environment class.
- Resource envelope.

If any field differs, the comparison is an experiment over that field.

Throughput claims include:

- Environment.
- Workload profile.
- Entropy class.
- Durability mode.
- Byte count.
- Total lifecycle duration.

## Analysis Record

- Hypothesis:
- Independent variable:
- Controlled variables:
- Measured variables:
- Result:
- Interpretation:
- Manifest path:
- Profile paths:
- Reproduction command:
