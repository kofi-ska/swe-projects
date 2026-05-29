# TxPool Builder v2 Test Plan

## Purpose
Prove the service is:
- deterministic
- bounded
- economical at `50,000 req/s`
- safe under failure
- cheap to migrate
- auditable under review

## Release Gates
Do not ship unless all required gates pass:
- determinism
- bound enforcement
- admission semantics
- snapshot epoch semantics
- selection correctness
- failure classification
- persistence durability
- migration compatibility
- degraded-mode recovery
- load and soak performance

## Evidence To Record
Every required run records:
- config digest
- policy version
- binary version
- snapshot ID
- candidate ID
- trace ID
- request ID
- queue depth
- build duration
- snapshot refresh duration
- artifact and trace sizes
- CPU, heap, and RSS peak
- RPC count
- shed count
- failure code

## Test Matrix

| Area | Test | Input | Assertion | Severity if Missing |
|---|---|---|---|---|
| Determinism | Stable replay | same config, same snapshot, same binary | same candidate ID, same selected order, same trace fingerprint | Critical |
| Determinism | Shuffled input | same txpool serialized with different map order | same output as stable replay | Critical |
| Determinism | Repeated run | run N times on identical snapshot | byte-stable selection and reason summary | Critical |
| Boundaries | Queue shed | queue at capacity | request is shed fast, not blocked | Critical |
| Boundaries | Job cap | retained job count exceeds limit | old jobs pruned, live jobs preserved | Critical |
| Boundaries | Snapshot cap | snapshot bytes exceed limit | capture fails closed | Critical |
| Boundaries | Artifact cap | artifact bytes exceed limit | write fails, job does not complete | Critical |
| Boundaries | Trace cap | trace bytes exceed limit | write fails, job does not complete | Critical |
| Admission | Idempotency required | missing idempotency key | request rejected | Critical |
| Admission | Duplicate idempotency | same idempotency key twice | same job or same result returned | Critical |
| Admission | Fresh snapshot required | stale or missing snapshot | request rejected or degraded per policy | Critical |
| Admission | Fast admission | steady load and burst load | admission latency stays within budget | Critical |
| Snapshot | Epoch identity | same head, same payload | same snapshot ID | Critical |
| Snapshot | Head drift | head changes during capture | strict mode fails, non-strict flags degraded | Critical |
| Snapshot | Atomic write | crash during snapshot write | no partial snapshot is visible | Critical |
| Snapshot | Retention safety | prune after refresh | current epoch is never deleted | Critical |
| Selection | Nonce gap | sender chain with gap | later txs are not selected | Critical |
| Selection | Replacement conflict | same sender and nonce | one tx survives, loser reason is stable | Critical |
| Selection | Gas bound | tx exceeds remaining gas | tx is excluded | Critical |
| Selection | Max tx bound | pool larger than max tx | selected count never exceeds limit | Critical |
| Selection | Empty pool | no eligible txs | empty candidate, not crash | High |
| Selection | All invalid | all txs fail decode or policy | empty output or hard failure per policy | High |
| Failure | RPC unavailable | upstream down | RPC failure code, no partial success | Critical |
| Failure | RPC schema drift | malformed `txpool_content` | schema error code, no silent fallback | Critical |
| Failure | Chain mismatch | wrong chain ID | startup failure, no build | Critical |
| Failure | Timeout | slow RPC or slow build | timeout code, bounded abort | Critical |
| Failure | Invariant failure | internal assertion violation | explicit invariant code | Critical |
| Persistence | Candidate write | normal write path | atomic rename, durable file | Critical |
| Persistence | Trace write | normal write path | atomic rename, durable file | Critical |
| Persistence | Disk denial | permission denied or full disk | job fails, no corruption | Critical |
| Persistence | Crash recovery | crash mid-write | temp files do not affect next run | Critical |
| Migration | Snapshot compatibility | old snapshot version | readable or explicit migration error | Critical |
| Migration | Candidate compatibility | old candidate version | replay reads or fails clearly | Critical |
| Migration | Trace compatibility | old trace version | replay reads or fails clearly | Critical |
| Migration | Config compatibility | old config file | loads with explicit defaults or clear error | High |
| Migration | Unknown fields | extra fields in stored JSON | ignored or rejected by policy, never misread | High |
| Degraded | Snapshot stale | fresh window exceeded | service marks degraded, does not lie healthy | Critical |
| Degraded | Queue saturation | sustained overload | service sheds, does not thrash | Critical |
| Degraded | Refresh failure | repeated refresh errors | service stays up, mode degrades | Critical |
| Degraded | Recovery | upstream recovers | service returns to normal mode | High |
| Performance | Admission p99 | target load and burst load | p99 within budget | Critical |
| Performance | Build p99 | small and large snapshots | p99 within budget | Critical |
| Performance | Refresh p99 | normal and pressured refresh | p99 within budget | Critical |
| Performance | Replay p99 | replay path only | replay cheaper than live build | High |
| Performance | Memory soak | 10 to 60 minute run | heap and RSS stabilize | Critical |
| Performance | GC pressure | high-QPS run | pause time and allocation rate bounded | High |
| Performance | Lock contention | many admissions and status calls | no global lock hotspot | High |
| Performance | Queue behavior | burst and overload | backlog bounded, no deadlock | Critical |
| Risk | Retry storm | repeated timeout retries | no amplification loop | Critical |
| Risk | Duplicate storm | repeated same idempotency key | request coalesces, cost stays bounded | Critical |
| Risk | Snapshot invalidation cascade | refresh invalidates active epoch | current jobs complete or fail cleanly | Critical |
| Risk | Hot sender chain | one sender with long nonce chain | selection remains bounded | High |
| Risk | Large trace pressure | max trace size input | trace cap enforced, no memory blowup | Critical |
| Risk | Slow disk | throttled filesystem | persistence degrades without corrupting state | High |
| Risk | Slow upstream | RPC latency spike | service sheds or degrades, no hang | Critical |
| Risk | Crash during refresh | process terminated mid-refresh | restart stays consistent | Critical |

## Performance Budgets To Assert
Record and enforce:
- admission latency p50 / p95 / p99
- build completion latency p50 / p95 / p99 / p999
- refresh latency p50 / p95 / p99
- replay latency p50 / p95 / p99
- heap peak
- RSS peak
- allocation rate
- GC pause time
- queue depth peak
- shed rate
- RPC calls per admitted request
- artifact bytes per job
- trace bytes per job
- snapshot bytes per refresh

## Risk Assertions
The suite must prove:
- failures are classified, not blurred
- overload sheds instead of amplifying
- stale state is visible, not hidden
- retries do not create a positive feedback loop
- crash recovery does not corrupt retained state
- migration does not break old retained artifacts
- deterministic replay stays stable across runs

## Required Load Runs
Run these before release:
- steady-state at target QPS
- 2x burst for short window
- 5x burst for short window
- 10x burst for short window
- 10 to 60 minute soak
- retry storm
- duplicate storm
- slow RPC run
- slow disk run
- replay-only run

## Exit Criteria
The release is blocked if any of these fail:
- same input does not produce same output
- any bound can grow without a hard stop
- any failure is misclassified
- artifact or trace writes can corrupt state
- replay cannot compare like-for-like artifacts
- sustained load causes unbounded memory growth
- overload causes queue collapse or retry amplification
- migration breaks retained artifacts or live upgrades

