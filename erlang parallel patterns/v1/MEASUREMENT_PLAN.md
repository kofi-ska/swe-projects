# Measurement Plan

This document defines how to validate the Erlang parallel-pattern runtime.

Scope:

- `farm`
- `pipeline`
- `fork-join`
- region lifecycle
- bounded queues and inflight work
- retries, timeouts, cancellation, and late results
- distributed placement and node failure handling
- observability overhead
- orchestration overhead
- fairness under contention
- Docker Compose validation before k3s validation
- measurement artifact capture in MinIO-compatible object storage

## 1. Claims to Validate

### 1.1 Lifecycle

- Each submitted work item reaches exactly one terminal state.
- Terminal states are `completed`, `failed`, `timed_out`, `cancelled`, or `rejected`.
- Cancel, timeout, retry, and late-result handling are idempotent.
- Region drain prevents new admissions and allows in-flight work to settle or time out.

### 1.2 Boundedness

- Queue depth is bounded at every boundary.
- Inflight work is bounded at every boundary.
- Retry budgets are bounded and enforced.
- Fan-out and stage buffers are bounded and enforced.
- Message size limits are enforced before dispatch.

### 1.3 Pattern Semantics

- `farm` executes independent tasks without default ordering.
- `pipeline` propagates backpressure upstream.
- `fork-join` splits into a finite child set and joins on the required completion set.

### 1.4 Recovery

- Worker failure is localized.
- Node failure is detected and reclassified.
- Late completions after terminal settlement are ignored safely.
- Retry exhaustion ends work without duplicate settlement.

### 1.5 Observability

- Queue delay and execution time are measured separately.
- Admission, reject, defer, retry, timeout, cancel, completion, and failure rates are visible.
- Region, coordinator, worker, stage, join, and node health are visible.
- Trace correlation survives local and remote execution.
- Structured lifecycle logs are queryable in Loki without using high-cardinality IDs as labels.

### 1.6 Economics

- Orchestration overhead stays bounded relative to task value.
- Retry amplification stays near `1.x` under normal load.
- Remote placement is used only when policy and capacity make it worthwhile.
- Measurement artifacts are written outside the runtime critical path and are not used as job durability.

## 2. Test Environment

### 2.1 Baseline environment

- Erlang/OTP version fixed and recorded.
- Single-node run and distributed run executed separately.
- Same build artifact for all runs.
- Same configuration snapshot for all repeats.
- Clock source synchronized across nodes.
- Docker Compose run executed before k3s validation.
- k3s run uses the same runtime image and equivalent configuration.
- No Cassandra-backed durability is present in v1 runs.
- No Grafana dependency is required for validation.

### 2.2 Required telemetry

- timestamps in monotonic time
- region/job/task IDs
- attempt number
- worker ID
- node ID
- trace ID
- queue depth
- inflight count
- terminal state
- rejection reason
- log event name
- artifact URI for persisted reports where relevant

### 2.3 Measurement rules

- Warmup run before recording.
- Fixed run duration per test.
- Repeated runs for each scenario.
- Report median and spread across runs.
- Separate local execution metrics from remote execution metrics.
- Separate queue delay from execution time.
- Record the exact config used for each run.
- Store measurement reports, benchmark outputs, config snapshots, and fault-injection summaries in MinIO-compatible object storage.
- Do not read runtime settlement state from MinIO during validation.

Recommended minimum:

- warmup: 30s
- run: 5m
- repeats: 5
- report: min / median / p95 / p99

## 3. Metrics

### 3.1 Core latency

- `submit_to_admit`
- `admit_to_start`
- `start_to_complete`
- `submit_to_complete`
- `queue_delay = admit_to_start`
- `execution_time = start_to_complete`

### 3.2 Capacity

- `queue_depth`
- `queue_growth_rate`
- `queue_age_p95`
- `queue_age_p99`
- `inflight_count`
- `worker_utilization`
- `stage_utilization`
- `join_active_child_count`

### 3.3 Control

- `admission_rate`
- `reject_rate`
- `defer_rate`
- `retry_rate`
- `retry_amplification`
- `timeout_rate`
- `cancel_rate`
- `completion_rate`
- `failure_rate`

### 3.4 Health

- `worker_health`
- `stage_health`
- `coordinator_health`
- `region_health`
- `node_health`
- `mailbox_depth`
- `scheduler_run_queue`

### 3.5 Cost

- `orchestration_overhead_pct`
- `telemetry_overhead_pct`
- `remote_dispatch_ratio`
- `bytes_per_task`
- `messages_per_task`
- `spans_per_task`

### 3.6 Artifacts

- `artifact_write_success_rate`
- `artifact_write_latency`
- `artifact_bytes_written`
- `report_uri`
- `config_snapshot_uri`
- `fault_summary_uri`

## 4. Workload Model

### 4.1 Task classes

- small task: short CPU-bound payload
- medium task: medium CPU-bound payload
- long task: long CPU-bound payload
- I/O-light task: mostly message passing, low CPU
- large payload task: high serialization cost

### 4.2 Arrival patterns

- steady arrival
- burst arrival
- skewed tenant arrival
- mixed priority arrival
- adversarial duplicate arrival
- adversarial oversized-message arrival

### 4.3 Concurrency profiles

- low: below 25% of configured capacity
- target: 50% to 70% of configured capacity
- high: 80% to 100% of configured capacity
- overload: above configured capacity

### 4.4 Scale profiles

- 1 node, 1 worker group
- 1 node, many workers
- Docker Compose: 1 coordinator, many worker containers
- many nodes, bounded worker groups
- many nodes, skewed load
- k3s: 1 coordinator deployment, scalable worker deployment

## 5. Experiment Matrix

### 5.1 Lifecycle correctness

Purpose:

- prove terminal-state correctness
- prove idempotence of cancel, retry, timeout, and late-result handling

Procedure:

- submit a fixed set of work items
- vary completion order
- inject cancel, retry, and timeout events
- inject late completions after terminal state

Pass criteria:

- each work item ends in exactly one terminal state
- no duplicate settlement
- no missing settlement
- late arrivals do not reopen terminal work

### 5.2 Bounded queues

Purpose:

- prove bounded queue enforcement at region, coordinator, stage, and worker boundaries

Procedure:

- drive arrival rate above configured capacity
- hold worker throughput below arrival rate
- continue until queue pressure triggers admission control

Pass criteria:

- queue depth never exceeds configured bound
- inflight count never exceeds configured bound
- excess demand is rejected, deferred, or backpressured explicitly
- queue growth rate does not remain positive in steady state after pressure is removed

### 5.3 Farm dispatch

Purpose:

- prove worker selection, unordered default execution, bounded inflight control

Procedure:

- submit independent tasks with mixed durations
- vary worker health and capacity
- compare local and remote placement

Pass criteria:

- tasks complete independently
- no default ordering guarantee appears
- selection respects policy and health
- failure stays localized to worker scope

### 5.4 Pipeline flow control

Purpose:

- prove stage buffering, stage concurrency, and backpressure propagation

Procedure:

- attach a slow downstream stage
- hold upstream arrival constant
- vary stage concurrency and buffer limits

Pass criteria:

- upstream pressure increases when downstream capacity drops
- stage buffers stay bounded
- stage hop latency is measurable separately
- ordered behavior appears only when configured

### 5.5 Fork-join settlement

Purpose:

- prove bounded fan-out, join completeness, reduction correctness

Procedure:

- split parent jobs into fixed child counts
- inject child failure, timeout, retry, and late completion
- vary reducer behavior and failure policy

Pass criteria:

- fan-out never exceeds `Fmax`
- join completes only when required child set settles
- reduction result matches expected output
- duplicate and late child results do not corrupt settlement

### 5.6 Recovery under worker failure

Purpose:

- prove worker isolation and recovery

Procedure:

- kill workers mid-task
- kill workers before dispatch
- restart workers during active load

Pass criteria:

- in-flight work is reclassified by policy
- work is retried, failed, or isolated according to policy
- no duplicate final settlement occurs

### 5.7 Recovery under node failure

Purpose:

- prove distributed failure handling

Procedure:

- terminate a remote node
- delay node rejoin
- reintroduce the node

Pass criteria:

- node loss is detected
- tasks on the lost node are reclassified
- late remote messages do not corrupt terminal state
- recovery does not require global restart

### 5.8 Observability under stress

Purpose:

- prove that metrics, logs, traces, and measurement artifacts remain usable under load

Procedure:

- run at target load and overload
- enable normal telemetry sampling
- inspect telemetry delay and drop behavior
- query lifecycle logs in Loki
- inspect traces in Jaeger
- inspect metrics in Prometheus
- upload run reports and config snapshots to MinIO

Pass criteria:

- metric freshness remains acceptable
- trace correlation remains intact for sampled traces
- logs remain structured and queryable
- telemetry overhead stays bounded
- MinIO artifact writes succeed without affecting admission, dispatch, or terminal settlement
- high-cardinality IDs appear in log bodies and reports, not as required Loki labels

### 5.9 Economics

Purpose:

- prove orchestration cost stays bounded

Procedure:

- run small tasks, medium tasks, and large tasks
- compare useful work time against orchestration time
- compare local and remote placement cost

Pass criteria:

- overhead percentage remains within configured target
- retry amplification remains near `1.x` in normal load
- remote placement is used only when it improves utility under policy

### 5.10 Docker Compose deployment

Purpose:

- prove that the first containerized distributed target supports runtime semantics and observability

Procedure:

- run one coordinator and multiple worker containers
- run OTEL Collector, Jaeger, Prometheus, Loki, log shipping, and MinIO
- submit work for each supported pattern
- kill and restart worker containers during active load
- collect reports, logs, traces, and metrics for each run

Pass criteria:

- Erlang nodes connect predictably through configured node names, cookies, and ports
- worker container failure is detected and reclassified
- no duplicate terminal settlement occurs after container restart
- Jaeger receives sampled traces
- Prometheus scrapes runtime metrics
- Loki receives structured lifecycle logs
- MinIO stores run artifacts

### 5.11 k3s deployment

Purpose:

- prove that the runtime contract survives Kubernetes-style scheduling and rollout behavior

Procedure:

- deploy coordinator and worker workloads to k3s using the same image as Compose
- configure Erlang cookie, runtime limits, telemetry endpoints, and MinIO credentials through Kubernetes resources
- run readiness, liveness, scaling, and rolling restart scenarios
- trigger graceful drain through pod termination

Pass criteria:

- pod DNS and Services support Erlang distribution
- worker scaling does not bypass admission or budget checks
- readiness becomes false during drain or overload
- `SIGTERM` starts graceful drain where possible
- rolling restarts do not create duplicate terminal settlement
- observability and artifact capture remain available after pod restarts

## 6. Fault Injection Matrix

### 6.1 Process faults

- kill worker during execution
- kill coordinator during active region
- kill stage worker during pipeline hop
- kill join aggregator during join settlement

### 6.2 Node faults

- terminate remote node
- restart remote node
- delay node rejoin
- terminate worker container
- terminate worker pod
- roll coordinator pod during drain

### 6.3 Message faults

- duplicate submit
- duplicate completion
- late completion
- oversized message
- malformed message
- delayed message

### 6.4 Load faults

- burst submit
- tenant skew
- retry storm
- mixed-size payload flood
- overload above capacity

### 6.5 Observability and artifact faults

- OTEL Collector unavailable
- Jaeger unavailable
- Prometheus scrape delayed
- Loki unavailable
- MinIO unavailable
- artifact upload delayed

## 7. Acceptance Criteria

The implementation passes only if all of the following hold:

- every submitted item reaches exactly one terminal state
- no queue or inflight bound is exceeded
- no retry budget is exceeded
- no fan-out bound is exceeded
- no stage buffer bound is exceeded
- queue delay and execution time are separately measurable
- p95 and p99 latency are recorded for each latency class
- retry amplification stays near `1.x` under normal load
- overwrite or replay does not create duplicate terminal settlement
- late arrivals do not reopen terminal work
- worker and node failure are localized
- observability remains usable under stress
- overhead remains bounded relative to useful work
- remote placement does not bypass admission control
- MinIO artifacts are written for completed measurement runs
- loss or delay of observability backends does not alter terminal settlement semantics
- no Cassandra-backed durability is required

## 8. Reporting Format

Each run reports:

- runtime version
- OTP version
- configuration snapshot
- topology
- load profile
- fault injection set
- pattern under test
- metrics summary
- log query references
- trace references
- artifact URIs
- pass/fail by claim
- failures with timestamps and IDs
- deltas from prior run

## 9. Execution Order

1. Validate single-node lifecycle correctness.
2. Validate boundedness at low and target load.
3. Validate pattern semantics.
4. Validate recovery under process faults.
5. Validate Docker Compose with coordinator, workers, Jaeger, Prometheus, Loki, and MinIO.
6. Validate recovery under node and container faults.
7. Validate observability and artifact capture under stress.
8. Validate economics under small, medium, and large tasks.
9. Validate k3s only after Compose results are stable.
10. Validate broader distributed runs only after single-node and Compose results are stable.

## 10. Result

The architecture is validated only when the measurements show:

- boundedness
- terminal-state correctness
- predictable degradation
- recovery without duplicate settlement
- measurable observability
- queryable lifecycle logs
- persisted measurement artifacts
- bounded orchestration cost
- pattern-specific behavior under load
