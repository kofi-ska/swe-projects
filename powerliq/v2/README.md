# PowerLiq V2

PowerLiq V2 is the distributed, event-driven evolution of PowerLiq. AI is outside its scope.

The V2 foundation establishes a Maven reactor, extracts the deterministic V1 financial engine into a framework-independent library, defines versioned event contracts and creates independently buildable service boundaries. Business behavior will be added incrementally rather than copied from the V1 monolith.

## Modules

- `libraries/financial-core`: value types, settlement-interval rules, valuation, collateral, liquidity and scenario calculations.
- `contracts/event-contracts`: versioned, transport-neutral domain-event contracts.
- `services/position-service`: position lifecycle owner.
- `services/portfolio-service`: portfolio cash and liquidity-setting owner.
- `services/market-ingestion-service`: scheduled ingestion and backfill owner.
- `services/market-data-service`: normalized price and revision owner.
- `services/valuation-service`: liquidity projection and scenario owner.
- `tests/architecture`: automated peer-service dependency restrictions.

## Build

Java 21 is required.

```shell
./mvnw test
```

## Local infrastructure

Docker Compose provides bounded local-only infrastructure:

- One PostgreSQL container with a logically separate database for each service.
- LocalStack emulation for a two-shard Kinesis stream plus bounded retry and dead-letter queues.

```shell
docker compose up -d --wait
docker compose down
```

These resources support development and tests only. Production AWS resources will be owned by Terraform under `deploy/v2`.
