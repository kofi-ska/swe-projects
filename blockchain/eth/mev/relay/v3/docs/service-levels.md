# v3 Service Levels

This is the internal service-level contract for v3. It binds the math to the runtime. There is no published external SLA.

## Scope

- Public contract: mev-boost compatibility and safe rejection
- Internal contract: bounded latency, bounded backlog, fenced authority, deterministic recovery, adaptive policy updates

## SLIs

| SLI | Source | Meaning |
|---|---|---|
| `queue_age_p95`, `queue_age_p99` | metrics | tail waiting time |
| `queue_age_max` | metrics | unsafe boundary check |
| `authority_margin` | lease timing | time left after renew jitter and skew |
| `stale_writer_reject_rate` | metrics | fence enforcement working |
| `mixed_authority_count` | health / logs | split-brain indicator |
| `retry_debt_peak` | metrics | retry backlog under failure |
| `terminalization_latency_p95` | metrics | durable closeout cost |
| `recovery_duration_p95` | measurement | shard return-to-ready time |
| `ready_ratio` | health | availability proxy |

## SLOs

| SLO | Target |
|---|---|
| Queue age | `queue_age_p99 < 1s` in normal load; `queue_age_max < 12s`; `>= 12s` is unsafe |
| Authority | `authority_margin > 0` for every live shard |
| Stale writers | stale authority acceptance = `0` |
| Mixed authority | mixed authority count = `0` |
| Retry debt | bounded under burst; no unbounded growth |
| Recovery | replay either rejoins after validation or quarantines; no partial live reuse |
| Terminalization | remains bounded and does not block the hot path indefinitely |
| Control loop | policy revision changes only when measured pressure or confidence changes |

## MTTR

MTTR is measured per failure class from first unsafe signal to Ready.

| Failure class | MTTR definition |
|---|---|
| Queue saturation | time until queue age and depth return under target and the shard is Ready |
| Authority loss | time until a fresh lease, epoch, and fence are in place and writes are accepted again |
| Recovery failure | time until the shard is either quarantined or rejoined with valid replay state |
| Backend outage | time until backend health is green and admission can resume |
| Rollout failure | time until one owner remains and the shard is either on the prior safe version or safely cut over |

## Math binding

These service levels are read against the spec equations:

- `λ_eff,s = λ_in,s + λ_retry,s + λ_dup,s`
- `μ_s = W_s / E[T_s]`
- stable iff `λ_eff,s < μ_s`
- `SafeAuthorityMargin = τ - ρ - δ`
- `ExpectedNet = ExpectedValue - DelayCost - ComputeCost - RetryCost - FailureRisk`

If a measured run violates one of those inequalities, the shard is not healthy even if the process is still up.

## SLA boundary

No external SLA is published yet.

The only external promise is protocol compatibility and safe rejection of stale or unsafe work.
