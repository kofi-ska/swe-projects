# PowerLiq V1

PowerLiq V1 is a deterministic GB electricity-liquidity REST application.

It provides a Java financial engine, a versioned Spring Boot API, a live Elexon Market Index check/synchronisation client, and PostgreSQL persistence managed by Flyway.

## Financial conventions

- Currency: GBP.
- Price scale: 5 decimal places, GBP/MWh.
- Energy scale: 3 decimal places, MWh.
- Money scale: 2 decimal places, GBP.
- Rounding: `HALF_EVEN`.
- A scenario shock is expressed as a decimal fraction; `0.20` means a 20% increase.
- Scenario shocks are bounded to `-1.00` through `10.00`.
- Position volume is bounded to 1,000,000 MWh.
- Portfolio MTM is the sum of rounded position MTMs.
- Collateral is calculated per position as `max(0, -MTM)` and then summed without netting.

## Run tests

```shell
mvn test
```

The integration tests use Testcontainers and therefore require Docker.

## Run locally

```shell
cp .env.example .env
docker compose up -d
set -a
source .env
set +a
mvn spring-boot:run
```

## API V1

```text
POST /api/v1/positions
GET  /api/v1/positions
POST /api/v1/positions/{positionId}/settlement

GET  /api/v1/market-data/status
POST /api/v1/market-data/sync

GET  /api/v1/portfolios/{portfolioId}/liquidity
POST /api/v1/portfolios/{portfolioId}/scenarios
```

Position creation requires an `Idempotency-Key` header. Reusing a key with the same request returns the original position; reusing it with a different request returns `409`. Settlement and market-data synchronisation require `X-Admin-Key`. The configured V1 portfolio ID defaults to `10000000-0000-0000-0000-000000000001`.

## Operational configuration

`.env.example` contains only configuration used by the current application. Spring Boot does not load `.env` files automatically. For a local shell:

The actual `.env` and environment-specific variants are ignored by Git. Production database credentials and the admin key must come from a secret manager; the checked-in values are local-development defaults only.

The current controls cover database connection pooling, administrative writes, Elexon connection/read deadlines, Elexon response size, market-data freshness, graceful shutdown, health endpoints, log levels, portfolio values, and JVM container behaviour.

Runtime resource bounds are deliberately explicit:

- At most 1,000 positions, enforced transactionally in PostgreSQL.
- At most 10 database connections by default, with a 3-second acquisition deadline.
- At most 31 days in one Elexon synchronisation window.
- At most 5 MiB in one Elexon response by default, configurable up to 50 MiB.
- Market-data status checks are shared for 30 seconds instead of calling Elexon for every request.
- Only prices required by active positions are retained.

Liquidity and scenario reads use a repeatable-read transaction so positions and prices form one consistent snapshot. Responses identify whether market data is fresh, stale, or not required and include its effective timestamp.

Financial scale, rounding, settlement-period rules, and formulas are deliberately not runtime configuration. Changing an environment variable must not silently change authoritative financial results.

## V2 observability direction

V2 can add OpenTelemetry without changing the V1 API contract. The intended scope is W3C trace-context propagation, Spring MVC and Elexon client spans, PostgreSQL spans, trace-correlated structured logs, low-cardinality business metrics, exemplars, an OpenTelemetry Collector, explicit sampling policy, and attribute/cardinality budgets. No partial OTEL stack is included in V1.
