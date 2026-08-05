# Performance Engineering Specification and Baseline

## 1. Purpose

This document defines how the LLM gateway is benchmarked, which variables must be controlled, how results are interpreted, and which thresholds constitute a pass or regression.

It deliberately separates:

1. **Analytical expectations** derived from configuration.
2. **Go microbenchmarks** measured in-process.
3. **Service load tests** measured through HTTP.
4. **Stability and overload tests** measured through metrics and profiles.

Only the microbenchmark results in section 8 have been measured and recorded. The service-level result tables are templates until the corresponding controlled runs are performed. Functional deployment to k3s has been verified, but that is not evidence of load-test performance.

## 2. Performance questions

The test programme must answer the following questions independently.

| ID | Question | Primary evidence |
| --- | --- | --- |
| PQ-1 | What is the allocation and CPU cost of request serialization, batching, and token generation? | Go benchmarks and CPU/allocation profiles |
| PQ-2 | What sustained request and token throughput can one gateway replica deliver? | k6 plus Prometheus counters |
| PQ-3 | At what offered load do queue wait and time to first token breach their SLOs? | Prometheus histograms and k6 arrival rate |
| PQ-4 | Does overload remain bounded and recover without restart? | Queue rejections, heap, goroutines, readiness, recovery time |
| PQ-5 | How does batch size trade initial latency for throughput? | Controlled batch-size matrix |
| PQ-6 | Do long requests cause head-of-line blocking for short requests? | Mixed-token workload, segmented latency metrics |
| PQ-7 | Does cancellation promptly release request, worker, and goroutine resources? | Cancellation workload, active requests, goroutine profile |

## 3. System model and analytical envelope

### 3.1 Default parameters

| Parameter | Symbol | Default |
| --- | ---: | ---: |
| Worker count | `W` | 8 |
| Maximum batch size | `B` | 8 requests |
| Maximum batch wait | `D_batch` | 20 ms |
| Base inference delay | `D_base` | 50 ms |
| Per-token delay | `D_token` | 10 ms |
| Queue capacity | `Q` | 1,000 requests |
| Request timeout | `D_timeout` | 10 s |
| Stream write timeout | `D_write` | 1 s |

The fake worker processes requests within a batch concurrently. Batch completion is therefore determined by the longest active request in that batch.

For a homogeneous batch requesting `T` tokens:

```text
batch service time = D_base + T × D_token
ideal request throughput = W × B / batch service time
ideal token throughput = W × B × T / batch service time
```

These are upper bounds. They exclude queueing, batch formation, HTTP parsing, JSON encoding, socket writes, tracing, metrics, scheduling, and garbage collection.

### 3.2 Calculated upper bounds

| Tokens/request | Batch service time | Ideal requests/s | Ideal tokens/s | No-queue first token |
| ---: | ---: | ---: | ---: | ---: |
| 10 | 150 ms | 426.67 | 4,266.67 | 60–80 ms |
| 50 | 550 ms | 116.36 | 5,818.18 | 60–80 ms |
| 100 | 1.05 s | 60.95 | 6,095.24 | 60–80 ms |
| 500 | 5.05 s | 12.67 | 6,336.63 | 60–80 ms |

The no-queue first-token range is:

```text
D_base + D_token + [0, D_batch]
= 50 ms + 10 ms + [0, 20 ms]
= 60–80 ms
```

At 50 tokens, the 100 RPS target is approximately 86% of the ideal 116.36 RPS envelope. It is intentionally close enough to saturation to expose scheduler, serialization, telemetry, and queueing overhead. It should not be treated as guaranteed capacity until measured.

### 3.3 Concurrency and memory bounds

The application does not launch one processing goroutine for every offered request.

| Resource | Bound with defaults |
| --- | ---: |
| Queued requests | `Q` = 1,000 |
| Requests in the batcher | `B` = 8 |
| Requests buffered for workers | `W × B` = 64 |
| Requests actively processed | `W × B` = 64 |
| Worker request fan-out goroutines | `W × B` = 64 |
| Approximate admitted pipeline requests | `Q + B + 2 × W × B` = 1,136 |

The HTTP server may briefly create handlers for additional offered requests, but requests beyond pipeline capacity are rejected immediately with `503` and do not create processing goroutines.

Each admitted request has a one-element event channel. This bounds buffered token events while preserving backpressure. A blocked client is limited by `STREAM_WRITE_TIMEOUT`, and each batch request has its own bounded worker goroutine so one blocked stream does not prevent sibling requests from progressing.

## 4. SLIs and pass criteria

### 4.1 User-facing criteria

| SLI | Measurement | Pass criterion |
| --- | --- | --- |
| Availability | `success / (success + overloaded + timeout + internal_error)` | ≥ 99.9% during baseline; ≥ 99% at target load |
| Time to first token | `llm_gateway_time_to_first_token_seconds` | p95 < 300 ms |
| Inter-token delay | `llm_gateway_inter_token_duration_seconds` | p99 < 100 ms |
| End-to-end latency | `llm_gateway_request_duration_seconds` | p95 < 2 s for requests ≤100 tokens |
| Queue wait | `llm_gateway_queue_wait_seconds` | p99 < 200 ms |
| Target throughput | successful request rate | ≥100 RPS for 10 minutes at 50 tokens |
| Overload response | capacity rejection behavior | 100% explicit `503`; no indefinite waits |
| Recovery | overload removed to readiness restored | <30 s |

### 4.2 Resource-stability criteria

Use the final 80% of a sustained run after warm-up.

| Resource | Pass criterion |
| --- | --- |
| Goroutines | No positive linear trend greater than 1 goroutine/minute |
| Live heap | Final 5-minute mean ≤10% above first post-warm-up 5-minute mean |
| RSS | No monotonic growth across three consecutive 5-minute windows |
| Queue depth | Returns below 10% capacity within 30 seconds after overload ends |
| Active requests | Returns to zero after load plus request timeout |
| Cancellation | ≥99% of cancelled requests release active work within 1 second |

Growth thresholds are engineering tripwires, not universal production limits. Any breach requires a heap or goroutine profile and a repeated run before classification as a leak.

### 4.3 Microbenchmark regression criteria

Compare changes using at least 10 samples per revision and `benchstat`.

Fail a change when either condition holds without an explained tradeoff:

- Time/op regresses by more than 10% with statistical significance.
- Allocations/op increase by one or more on a hot path.
- Bytes/op increase by more than 10%.

## 5. Experimental controls

Every report must record:

- Git commit SHA and dirty-worktree status.
- Go version, OS, architecture, CPU model, and logical CPU count.
- Deployment mode: local process, Docker Compose, or k3s.
- Replica count and pod CPU/memory requests and limits.
- All gateway configuration values.
- Trace sampling rate and failure rate.
- Load-generator version, location, and available CPU.
- Start/end timestamps and test duration.
- Whether images and dependencies were already warm.

Mandatory controls:

1. Set `FAILURE_RATE=0` unless fault injection is the variable under test.
2. Hold prompt length constant at 27 ASCII characters unless prompt parsing is under test.
3. Use fixed token counts for homogeneous tests.
4. Do not change batch size, worker count, replica count, and offered RPS in the same comparison.
5. Run a 2-minute warm-up before collecting a sustained-load sample.
6. Perform at least three service-level runs per configuration.
7. Report the median run and the range across runs; do not select the best run.
8. Keep `OTEL_SAMPLE_RATE` fixed. Recommended: `0.1` for load tests and `1.0` for trace-shape validation.
9. Run the load generator outside the gateway pod. Confirm the generator itself is not CPU-saturated.

## 6. Workload matrix

### 6.1 Required service scenarios

| ID | Scenario | Offered load | Tokens | Duration | Purpose |
| --- | --- | ---: | ---: | ---: | --- |
| LT-01 | Warm-up | 25 RPS | 50 | 2 min | Stabilize code, connections, heap, and collectors |
| LT-02 | Baseline | 25 RPS | 50 | 5 min | Establish low-contention latency and resource floor |
| LT-03 | Target | 100 RPS | 50 | 10 min | Validate target throughput and SLOs |
| LT-04 | Capacity sweep | 25, 50, 75, 100, 125, 150 RPS | 50 | 3 min/step | Locate queueing knee and maximum sustainable rate |
| LT-05 | Overload | Increase until queue full | 50 | 5 min + 2 min recovery | Validate bounded rejection and recovery |
| LT-06 | Short requests | 100 RPS | 10 | 5 min | Measure scheduler and fixed-cost ceiling |
| LT-07 | Long requests | 10 RPS | 500 | 5 min | Validate timeout margin and worker occupancy |
| LT-08 | Mixed lengths | 40% 10, 30% 50, 20% 100, 10% 500 | mixed | 10 min | Detect head-of-line effects |
| LT-09 | Cancellation storm | 100 RPS; cancel 25% after first token | 50 | 5 min | Verify prompt cancellation and no goroutine retention |
| LT-10 | Soak | 50 RPS | 50 | 60 min | Detect slow heap, RSS, or goroutine growth |

### 6.2 Batch-size experiment

For each `MAX_BATCH_SIZE` in `1, 2, 4, 8, 16`, hold the following constant:

```text
WORKER_COUNT=8
MAX_BATCH_WAIT=20ms
BASE_INFERENCE_DELAY=50ms
PER_TOKEN_DELAY=10ms
TOKENS=50
FAILURE_RATE=0
OTEL_SAMPLE_RATE=0.1
```

Run 25, 75, and 100 RPS for five minutes each. Record:

- Achieved successful RPS and tokens/s.
- Mean and p95 batch size.
- p50/p95/p99 queue wait.
- p50/p95/p99 time to first token.
- p50/p95/p99 request duration.
- CPU cores, RSS, live heap, allocation rate, and goroutines.
- `503`, timeout, cancellation, and internal-error rates.

The selected batch size must be Pareto-efficient: no alternative may improve throughput without worsening p95 time to first token, or improve p95 time to first token without reducing throughput.

## 7. Execution procedures

### 7.1 Correctness gates

Run before any performance measurement:

```bash
go test ./...
go test -race ./...
go vet ./...
```

A benchmark from a revision that fails any correctness gate is invalid.

### 7.2 Repeatable Go microbenchmarks

Capture ten samples with a three-second minimum per benchmark:

```bash
mkdir -p results
go test -run='^$' -bench=. -benchmem -benchtime=3s -count=10 ./internal/... \
  | tee results/microbench.txt
```

For a before/after comparison:

```bash
git switch <baseline-revision>
go test -run='^$' -bench=. -benchmem -benchtime=3s -count=10 ./internal/... > results/before.txt

git switch <candidate-revision>
go test -run='^$' -bench=. -benchmem -benchtime=3s -count=10 ./internal/... > results/after.txt

benchstat results/before.txt results/after.txt
```

Do not compare single samples or results collected with different Go versions.

### 7.3 k6 service test

The provided k6 scenario uses a constant arrival rate. Example target run:

```bash
k6 run \
  -e BASE_URL=http://localhost:8080 \
  -e RPS=100 \
  -e DURATION=10m \
  -e TOKENS=50 \
  --summary-export=results/target-summary.json \
  loadtests/generate.js
```

k6's HTTP timing represents the completed response, not server-observed token boundaries. Use Prometheus for time-to-first-token and inter-token SLIs.

### 7.4 Prometheus queries

Run these over the exact test window.

Successful RPS:

```promql
sum(rate(llm_gateway_requests_total{outcome="success"}[1m]))
```

Availability:

```promql
sum(rate(llm_gateway_requests_total{outcome="success"}[5m]))
/
sum(rate(llm_gateway_requests_total{outcome=~"success|overloaded|timeout|internal_error"}[5m]))
```

p95 time to first token:

```promql
histogram_quantile(0.95,
  sum by (le) (rate(llm_gateway_time_to_first_token_seconds_bucket[5m])))
```

p99 queue wait:

```promql
histogram_quantile(0.99,
  sum by (le) (rate(llm_gateway_queue_wait_seconds_bucket[5m])))
```

Average batch size:

```promql
sum(rate(llm_gateway_batch_size_sum[5m]))
/
sum(rate(llm_gateway_batch_size_count[5m]))
```

Queue utilization:

```promql
llm_gateway_queue_depth / llm_gateway_queue_capacity
```

### 7.5 Profiles

Capture profiles during the steady-state portion of the same load run:

```bash
curl -o results/cpu.pprof 'http://localhost:6060/debug/pprof/profile?seconds=30'
curl -o results/heap.pprof http://localhost:6060/debug/pprof/heap
curl -o results/allocs.pprof http://localhost:6060/debug/pprof/allocs
curl -o results/goroutine.pprof http://localhost:6060/debug/pprof/goroutine
curl -o results/mutex.pprof 'http://localhost:6060/debug/pprof/mutex?seconds=30'
curl -o results/block.pprof 'http://localhost:6060/debug/pprof/block?seconds=30'

go tool pprof -top results/cpu.pprof
go tool pprof -top -cum results/heap.pprof
go tool pprof -http=:8081 results/cpu.pprof
```

For k3s, first forward the administrative port directly from one gateway pod:

```bash
kubectl -n llm-gateway port-forward deployment/gateway 6060:6060
```

Never expose port 6060 through a public Kubernetes Service.

## 8. Recorded microbenchmark baseline

### 8.1 Environment

| Field | Value |
| --- | --- |
| Date | 2026-08-05 |
| Git branch | `codex/llm-gateway` |
| Host | Apple M4 Pro |
| OS/architecture | Darwin/arm64 |
| Go | 1.25.4 |
| Method | `go test -bench=. -benchmem ./internal/...` |
| Samples | 1 |

Because only one sample was recorded, these values are an orientation baseline, not a statistically rigorous regression decision. Future comparisons must use the ten-sample procedure in section 7.2.

### 8.2 Hot-path results

| Benchmark | Time/op | Bytes/op | Allocations/op | Interpretation |
| --- | ---: | ---: | ---: | --- |
| Response serialization | 149.8 ns | 112 B | 2 | Cost to encode one NDJSON event into an in-memory buffer |
| Instrumented batcher | 836.1 ns | 1,040 B | 15 | Admission/batch overhead including test setup and tracing API calls |
| Token generation | 5.319 µs | 880 B | 12 | Generates and transfers 16 events through the bounded request stream |

The batcher benchmark measures enqueue/drain mechanics, not real batch service time. The worker benchmark uses zero configured inference delay so it isolates coordination and event-transfer overhead.

### 8.3 String aggregation optimization

This experiment isolates the allocation exercise for 500 generated tokens.

| Implementation | Time/op | Bytes/op | Allocations/op |
| --- | ---: | ---: | ---: |
| Repeated `result += token` | 188.730 µs | 1,105,899 B | 499 |
| Pre-sized `strings.Builder` | 2.933 µs | 4,864 B | 1 |

Calculated improvement:

| Metric | Calculation | Improvement |
| --- | --- | ---: |
| Execution time | `188,730 / 2,933` | 64.35× faster |
| Bytes allocated | `1,105,899 / 4,864` | 227.36× fewer bytes |
| Allocation count | `499 - 1` | 498 allocations removed (99.8%) |

The streaming endpoint does not concatenate its response. The optimized helper exists for non-streaming consumers and as a controlled demonstration of how allocation evidence guides implementation choice.

### 8.4 Core rewrite observation

An earlier single-sample token-generation benchmark measured 5.75 µs/op, 5,672 B/op, and 58 allocations/op. After request encapsulation and bounded fan-out were introduced, the recorded result was 5.319 µs/op, 880 B/op, and 12 allocations/op.

| Metric | Earlier | Current | Delta |
| --- | ---: | ---: | ---: |
| Time/op | 5.75 µs | 5.319 µs | −7.5% |
| Bytes/op | 5,672 B | 880 B | −84.5% |
| Allocations/op | 58 | 12 | −79.3% |

These single-sample values suggest a material allocation improvement but must be reproduced with `-count=10` before being presented as statistically established.

## 9. Service-level results

### 9.1 Current evidence status

| Evidence | Status |
| --- | --- |
| Unit tests | Passed |
| Race detector | Passed |
| `go vet` | Passed |
| Container image build | Passed |
| Two-node k3s deployment through k3d | Passed |
| Two gateway replicas ready | Passed |
| Streaming request through Kubernetes Service | Passed |
| Readiness through Kubernetes Service | Passed |
| Prometheus request/token metric emission | Passed |
| Controlled 5-minute baseline | Not yet recorded |
| Controlled 10-minute target test | Not yet recorded |
| Overload and recovery test | Not yet recorded |
| 60-minute soak | Not yet recorded |

### 9.2 Run-result template

| Field | Result |
| --- | --- |
| Scenario ID | |
| Commit SHA | |
| Start/end UTC | |
| Replicas | |
| Offered RPS | |
| Achieved successful RPS | |
| Tokens/request | |
| Tokens/s | |
| Availability | |
| TTFT p50 / p95 / p99 | |
| Inter-token p50 / p95 / p99 | |
| Request p50 / p95 / p99 | |
| Queue-wait p50 / p95 / p99 | |
| Mean / p95 batch size | |
| `503` / timeout / internal-error count | |
| Mean / peak CPU | |
| Mean / peak RSS | |
| Mean / peak live heap | |
| Initial / peak / final goroutines | |
| Recovery time | |
| Profile artifact paths | |
| Pass/fail and explanation | |

## 10. Analysis rules

1. Distinguish **offered RPS** from **achieved successful RPS**. A saturated generator does not prove gateway throughput.
2. Report latency percentiles with their request count. Percentiles from very small samples are not meaningful.
3. Separate `invalid` and `cancelled` outcomes from availability calculations.
4. Include `overloaded`, `timeout`, and `internal_error` in availability failures.
5. Do not average percentiles across replicas or runs. Aggregate histogram buckets for a test window, and report each run separately before selecting the median run.
6. Correlate a latency change with queue wait, batch size, CPU, and allocation rate before attributing cause.
7. Treat profiles as samples, not exact accounting. Repeat a profile if a suspected hotspot is close to sampling noise.
8. A throughput improvement that violates the time-to-first-token SLO is not an acceptable optimization.
9. A larger queue that reduces `503` responses by converting them into timeouts or excessive queue latency is a regression.
10. Preserve raw k6 summaries, Prometheus query outputs, configuration, and profiles under a timestamped `results/` directory; do not commit large binary profiles.

## 11. Completion criteria

The performance investigation is complete when:

- All required scenarios LT-01 through LT-10 have a recorded configuration and result.
- The target test passes or has a documented bottleneck with profile evidence.
- The overload test proves explicit rejection, bounded resources, and recovery.
- At least one before/after change has ten-sample microbenchmark results and repeated service-level results.
- CPU, heap, allocation, goroutine, mutex, and block profiles are captured for the target or overload workload.
- The selected worker and batch configuration is justified by the measured tradeoff matrix.
- Every numerical claim links to a raw result, Prometheus query, or profile artifact.
