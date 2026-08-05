# Go LLM Inference Gateway Simulator

Go service that simulates an LLM inference gateway with bounded admission, dynamic batching, a fixed worker pool, cancellable NDJSON streaming, metrics, traces, and profiling.

The model is simulated. No GPU or external model API is required.

## Architecture

```text
client -> HTTP validation -> bounded queue -> dynamic batcher -> worker pool
   ^                                                        |
   +-------------------- NDJSON token stream ----------------+
```

A full queue returns `503` with `Retry-After`. Worker request fan-out is bounded by `WORKER_COUNT × MAX_BATCH_SIZE`. Each request has a write deadline and a one-event buffer.

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

Ports: gateway `8080`, Grafana `3000`, Prometheus `9090`, Jaeger `16686`, loopback-only pprof `6060`.

## k3s

Build the image and import it into the local k3s containerd image store:

```bash
make k3s-import
make k3s-deploy
kubectl -n llm-gateway rollout status deployment/gateway
```

For k3s through k3d on macOS:

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

pprof is not exposed by the Kubernetes Service:

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

## Non-functional requirements

| ID | Requirement | SLI / acceptance target | Implementation |
| --- | --- | --- | --- |
| NFR-1 | Availability | ≥99.9% over 30 days | Final outcomes are `success`, `invalid`, `cancelled`, `overloaded`, `timeout`, or `internal_error`. Availability includes service-controlled failures and excludes invalid input and client cancellation. |
| NFR-2 | Initial response latency | TTFT p95 <300 ms over 7 days | Maximum batch wait, queue-wait histogram, first-token timestamp, fixed worker pool. |
| NFR-3 | Streaming responsiveness | Inter-token p99 <100 ms over 7 days | Per-token delay, token flush, one-event buffer, stream write deadline. |
| NFR-4 | End-to-end latency | p95 <2 s for requests ≤100 tokens | Request deadline, cancellable queue and worker contexts, duration histogram. |
| NFR-5 | Throughput | Sustain 100 successful RPS for 10 minutes at 50 tokens/request | Dynamic batches of up to 8 across 8 workers. This remains a test target, not a recorded result. |
| NFR-6 | Overload protection | 100% explicit capacity rejection; no indefinite wait | Queue capacity 1,000; non-blocking admission; `503` and `Retry-After`; readiness fails above 80% queue utilization. |
| NFR-7 | Resource efficiency | No sustained heap, RSS, or goroutine growth after warm-up | Bounded queue, fixed worker pool, maximum 64 worker fan-out goroutines with defaults, cancellable event delivery. |
| NFR-8 | Observability | Identify affected SLI and pipeline stage | Prometheus request/queue/batch/runtime metrics, OpenTelemetry spans, Jaeger, pprof. |
| NFR-9 | Privacy | No prompts, tokens, credentials, or user identifiers in telemetry | Only prompt length, token limit, priority, queue depth, batch size, worker ID, and token count are recorded. |
| NFR-10 | Operability | Health checks, graceful shutdown, dashboards, alerts, runbooks | Separate live/ready probes; admission closes before drain; 15-second shutdown deadline; provisioned operational assets. |

Availability calculation:

```text
success / (success + overloaded + timeout + internal_error)
```

The full measurement protocol and current evidence are in [docs/benchmark-results.md](docs/benchmark-results.md).

## Configuration

| Variable | Default |
| --- | --- |
| `HTTP_PORT` | `8080` |
| `PPROF_ADDRESS` | `localhost:6060` |
| `QUEUE_CAPACITY` | `1000` |
| `WORKER_COUNT` | `8` |
| `MAX_BATCH_SIZE` | `8` |
| `MAX_BATCH_WAIT` | `20ms` |
| `BLOCK_PROFILE_RATE` | `0` |
| `MUTEX_PROFILE_FRACTION` | `0` |
| `REQUEST_TIMEOUT` | `10s` |
| `STREAM_WRITE_TIMEOUT` | `1s` |
| `SHUTDOWN_TIMEOUT` | `15s` |
| `BASE_INFERENCE_DELAY` | `50ms` |
| `PER_TOKEN_DELAY` | `10ms` |
| `FAILURE_RATE` | `0.01` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `jaeger:4317` |
| `OTEL_SAMPLE_RATE` | `1.0` |
| `READINESS_SATURATION` | `0.8` |

Invalid values fail startup. Keep trace sampling fixed within a benchmark comparison.

Block and mutex profiling are disabled by default because they add overhead. The supplied Compose and Kubernetes configurations enable block rate `1` and mutex fraction `5` for profiling exercises.

## Diagnostics

| Question | Source |
| --- | --- |
| Are SLIs passing? | Grafana and Prometheus |
| Which pipeline stage is slow? | Jaeger |
| Which code consumes CPU or allocates memory? | pprof |
| Are goroutines blocked or retained? | Goroutine, block, and mutex profiles |
| What is the incident procedure? | `runbooks/` |
