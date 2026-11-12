# PowerLiq V2

PowerLiq V2 is a Java 21, distributed, event-driven electricity position and liquidity system. AI is intentionally outside its scope.

## Implemented services

- `position-service`: idempotent position creation, guarded settlement and transactional outbox.
- `portfolio-service`: cash and liquidity-buffer ownership with optimistic concurrency and transactional outbox.
- `market-ingestion-service`: UTC half-hour schedule, bounded historical backfills, durable run history, cancellation, deduplication and transient database retry.
- `market-data-service`: immutable price revisions, latest-price projection and idempotent EventBridge/SQS consumption.
- `valuation-service`: deterministic portfolio valuation, collateral/liquidity calculation and idempotent persisted results.

Each service is an independently buildable Spring Boot application with its own schema boundary. Shared modules contain transport-neutral contracts, deterministic financial logic, error contracts, OAuth2 resource-server policy and the outbox publisher. Architecture tests prohibit peer-service imports.

## Reliability and security

- PostgreSQL transactions atomically commit aggregates and outbox records.
- EventBridge and SQS provide at-least-once delivery; consumers deduplicate by event identity.
- Every external operation has bounded pools, timeouts, batches or retry counts controlled through environment variables.
- `/api/v2/**` uses Cognito-compatible JWT scopes in production; only health probes are anonymous.
- Errors use a sanitized shared envelope with stable service-specific codes and correlation IDs.
- CloudWatch receives bounded-retention logs, ECS/RDS/SQS metrics, a dashboard and SNS-backed alarms.

## Build and test

Java 21 and Docker are required.

```shell
./mvnw test
docker compose --env-file .env.example config --quiet
terraform -chdir=infra/terraform validate
```

For local infrastructure, copy `.env.example` to an untracked `.env`, replace every `change-me` value, then run:

```shell
docker compose up -d --wait
docker compose down
```

The optional operational profile builds Position and Market Data as independent containers and exercises real PostgreSQL and LocalStack EventBridge/SQS infrastructure:

```shell
docker compose --profile operational up -d --build --wait
POWERLIQ_LOAD_REQUESTS=500 POWERLIQ_LOAD_CONCURRENCY=20 ./tests/operational/load-position.sh
./tests/operational/verify-event-recovery.sh
docker compose --profile operational down
```

See [Operational verification](docs/operational-verification.md), [Capacity and economics](docs/capacity-and-economics.md), and the [AWS sandbox guide](infra/terraform/README.md). Production secrets belong in AWS Secrets Manager, never `.env` or Terraform variables.
