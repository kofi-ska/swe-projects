# Go LLM Inference Gateway Simulator

A production-style Go service that models the control plane around LLM inference: bounded admission, dynamic batching, a fixed worker pool, cancellable NDJSON streaming, Prometheus SLI metrics, OpenTelemetry traces, pprof, and explicit overload behavior.

The model is simulated. No GPU or external model API is required.

## Architecture

```text
client -> HTTP validation -> bounded queue -> dynamic batcher -> worker pool
   ^                                                        |
   +-------------------- NDJSON token stream ----------------+
```

The bounded queue and fixed number of workers are the central safety properties. A full queue produces an immediate `503` with `Retry-After`. Batch streaming uses at most `WORKER_COUNT × MAX_BATCH_SIZE` request goroutines, so a slow client cannot block its batch siblings and offered load cannot create unbounded worker fan-out.

## Run locally

Requirements: Go 1.24+.

```bash
make test
FAILURE_RATE=0 make run
```

In another terminal:

```bash
curl -N http://localhost:8080/v1/generate \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"Explain distributed tracing","max_tokens":10,"priority":"normal"}'
```

Endpoints:

| Endpoint | Purpose |
| --- | --- |
| `POST /v1/generate` | NDJSON generation stream |
| `GET /health/live` | Process liveness |
| `GET /health/ready` | Batcher, worker, saturation, and shutdown readiness |
| `GET /metrics` | Prometheus metrics |
| `localhost:6060/debug/pprof/` | Local administrative profiling |

## Docker Compose

```bash
docker compose up --build
```

This starts the gateway (`:8080`), Grafana (`:3000`), Prometheus (`:9090`), and Jaeger (`:16686`). pprof is bound to loopback at `:6060`. Dashboards and the Prometheus data source are provisioned automatically.

## k3s

Build the image and import it into the local k3s containerd image store:

```bash
make k3s-import
make k3s-deploy
kubectl -n llm-gateway rollout status deployment/gateway
```

On macOS, k3d runs k3s nodes in Docker. This repository has also been exercised with a one-server, one-agent k3d cluster:

```bash
make k3d-create
make k3d-import
make k3s-deploy
kubectl -n llm-gateway get pods -o wide
```

For multi-node k3s, push the image to a registry reachable by every node and replace `llm-gateway:local` plus `imagePullPolicy: Never` in `k8s/gateway.yml`.

Access the services:

```bash
kubectl -n llm-gateway port-forward service/gateway 8080:8080
kubectl -n llm-gateway port-forward service/grafana 3000:3000
kubectl -n llm-gateway port-forward service/prometheus 9090:9090
kubectl -n llm-gateway port-forward service/jaeger 16686:16686
```

pprof is deliberately absent from the Kubernetes Service. Open it only when needed:

```bash
kubectl -n llm-gateway port-forward deployment/gateway 6060:6060
go tool pprof http://localhost:6060/debug/pprof/heap
```

## Tests and measurement

```bash
make test
make race
make bench
```

The [benchmark baseline](docs/benchmark-results.md) records the machine and before/after allocation exercise. A k6 target-load scenario is included:

```bash
k6 run -e RPS=100 -e DURATION=10m -e TOKENS=50 loadtests/generate.js
```

For a dependency-free short run, use the Go load generator:

```bash
go run ./cmd/loadgen -rps 25 -duration 30s -tokens 50
```

## Reliability model

Every request ends with exactly one low-cardinality outcome: `success`, `invalid`, `cancelled`, `overloaded`, `timeout`, or `internal_error`. Invalid input and client cancellation are excluded from availability; service-controlled overload, timeout, and processing errors consume the error budget.

Initial targets are 99.9% availability over 30 days, p95 time to first token below 300 ms, p99 queue wait below 200 ms, and p95 completion below two seconds for requests up to 100 tokens. The SLO dashboard and alert rules encode those targets.

Prompts and generated text are never placed in logs, metrics, trace attributes, or profiling labels. Traces contain only bounded metadata such as prompt length, maximum token count, priority, queue depth, batch size, and worker ID.

## Configuration

| Variable | Default |
| --- | --- |
| `HTTP_PORT` | `8080` |
| `PPROF_ADDRESS` | `localhost:6060` |
| `QUEUE_CAPACITY` | `1000` |
| `WORKER_COUNT` | `8` |
| `MAX_BATCH_SIZE` | `8` |
| `MAX_BATCH_WAIT` | `20ms` |
| `REQUEST_TIMEOUT` | `10s` |
| `STREAM_WRITE_TIMEOUT` | `1s` |
| `SHUTDOWN_TIMEOUT` | `15s` |
| `BASE_INFERENCE_DELAY` | `50ms` |
| `PER_TOKEN_DELAY` | `10ms` |
| `FAILURE_RATE` | `0.01` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `jaeger:4317` |
| `OTEL_SAMPLE_RATE` | `1.0` |
| `READINESS_SATURATION` | `0.8` |

Invalid values fail startup. Use a lower trace sampling rate during sustained load.

## Operational investigation

Start with Grafana to confirm user impact, use Jaeger to locate time in queue/batch/inference/streaming, then use pprof to identify the responsible Go code. Change one variable, repeat the identical load scenario, and compare latency, throughput, allocation, heap, and goroutine trends. Runbooks under `runbooks/` cover queue saturation, latency, and error-rate incidents.
