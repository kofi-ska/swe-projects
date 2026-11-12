# Operational verification

## Acceptance gates

| Risk | Evidence | Pass condition |
|---|---|---|
| Financial or lifecycle regression | `./mvnw test` | Entire reactor passes, including PostgreSQL integration, concurrency, security and architecture tests |
| Configuration drift | `docker compose --env-file .env.example config --quiet` | Compose resolves without missing or malformed controls |
| Container viability | `docker compose --profile operational up -d --build --wait` | Independently built services reach readiness |
| Write load | `tests/operational/load-position.sh` | At least 99% HTTP 201 responses; no client call exceeds 10 seconds |
| Event dependency failure | `tests/operational/verify-event-recovery.sh` | Market Data remains ready while SQS is unavailable and projects an event after recovery |
| AWS structure | `terraform -chdir=infra/terraform validate` | Provider-backed validation succeeds |

## Latest local baseline

Measured on 12 November 2026 on a developer machine using the bounded Compose profile:

- 500 position writes at concurrency 20.
- 500 successes, zero failures (0.00% error rate).
- 1.686 seconds elapsed; 296.56 requests/second.
- Market Data readiness remained available during a LocalStack interruption.
- A valid market-price event was consumed and queryable after LocalStack restarted.

This is a repeatable smoke/load baseline, not an AWS production benchmark. Never use it to size production without running the same workload against the deployed instance class and realistic data volume.

## Failure semantics

- An unavailable database fails readiness and mutations fail without partial aggregate/outbox commits.
- EventBridge publication failures leave the outbox row unpublished and increment a bounded attempt count.
- SQS receive failures do not terminate Market Data; the scheduled poll resumes on the next bounded interval.
- A malformed queue message is not acknowledged. AWS redrive moves it to the DLQ after five receives, where the DLQ alarm pages the operator.
- Duplicate delivery is expected and safe; event identity and source revision uniqueness prevent duplicate projections.
- Ingestion cancellation is cooperative between source records. Interrupted runs become `CANCELLED`; source or timeout failures remain visible in run history.

## Alarm response

| Alarm | First checks | Safe action |
|---|---|---|
| Service unhealthy | ECS stopped reason, readiness logs, RDS connectivity | Roll back to the previous immutable image; do not bypass readiness |
| Target 5xx | Correlation IDs, recent deploy, DB saturation | Stop the faulty rollout; reduce callers only if saturation is proven |
| ECS CPU/memory | Request rate, heap/OOM logs, queue depth | Bound intake; resize only after profiling and budget review |
| RDS connections/storage | Pool totals, long transactions, storage trend | Kill verified abandoned work; increase storage before exhaustion |
| Market-price DLQ | Inspect one message without deleting it, correlate producer version | Fix/replay idempotently; never bulk-delete an unexplained DLQ |
| Budget 80% forecast | Cost Explorer by `Project=PowerLiq` | Stop nonessential tasks and log growth; require approval for a budget change |

## Recovery and RTO/RPO

The local dependency-recovery test confirms application resumption, not disaster recovery. The sandbox targets RTO under four hours and RPO under 24 hours from daily RDS backups. Those remain **unconfirmed objectives** until an AWS drill restores a snapshot into an isolated stack, validates row counts and checksums, replays pending events, and records timestamps.

For each AWS drill, record:

1. failure declaration time and last durable transaction time;
2. snapshot identifier and restore start/end;
3. schema migration result and per-table counts/checksums;
4. oldest pending outbox event and DLQ depth before/after replay;
5. first successful authenticated API request;
6. measured RTO and RPO, with an incident if either objective is missed.

Do not advertise confirmed RTO/RPO until this evidence exists for the deployed environment.
