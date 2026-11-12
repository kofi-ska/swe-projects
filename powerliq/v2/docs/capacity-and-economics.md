# Capacity and economics

## Workload model

The design assumption is **10 million retained domain events**, not 10 million events every 30 minutes. At a uniform yearly rate that is about 27,400 events/day or 0.32 events/second average. Corrections and backfills create bursts, so the system uses bounded batches, queues and idempotent replay rather than sizing only for the average.

Conservative planning assumptions:

- 1.5–2.5 KiB per event after PostgreSQL row, JSON payload and index overhead.
- 10 million events therefore require roughly 15–25 GiB before temporary migration, vacuum and growth headroom.
- The sandbox RDS range of 20–30 GiB is suitable only for demonstration and early measurement; it is unsafe as a fully loaded 10-million-event production target.
- A production capacity gate should retain at least 40% free storage and test query plans at the target cardinality.

## Bounded resources

- HTTP worker, accept and connection counts are finite per service.
- Hikari pools are finite and collectively must stay below the RDS connection alarm threshold.
- API list/history ranges are bounded.
- Backfills default to 31 days per request.
- Publisher and consumer batches default to 10.
- Database retry defaults to three attempts with short linear delay and applies only to transient failures.
- SQS long polling and visibility timeout are bounded; poison events redrive after five receives.
- CloudWatch retention, ECR image count and RDS autoscaling maximum are bounded.

## Sandbox unit economics

The Terraform baseline is estimated at USD 74/month in `eu-west-2`:

| Component | Planning estimate/month |
|---|---:|
| Single Graviton ECS EC2 host | USD 25 |
| Single-AZ RDS PostgreSQL and 20 GiB | USD 16 |
| Application Load Balancer | USD 18 |
| Logs, metrics, storage, ECR and transfer allowance | USD 15 |
| Total | **USD 74** |

Terraform rejects a configured budget above USD 100 and compute/database classes larger than the approved sandbox bounds. AWS Budgets alerts at 80% forecast and 100% actual. Prices and usage vary, so Cost Explorer remains authoritative.

The low cost is purchased with explicit availability trade-offs: one ECS node, single-AZ RDS, no NAT Gateway and no provisioned multi-AZ failover. This is a sandbox, not a production HA claim.

## Scale decision

Before 10 million retained events, run a production-shaped test with the final indexes and retention policy. Scale or partition only when evidence shows one of these thresholds:

- p95 latency or error SLO is missed under representative bursts;
- RDS CPU is sustained above 70%, connections above 70, or free storage below 40%;
- autovacuum cannot keep dead tuples bounded;
- backfill lag cannot recover before the next scheduled interval;
- queue age grows continuously while consumers are healthy.

Prefer index/query correction and retention/archival before adding brokers or services. Kafka/Kinesis, multi-AZ RDS and autoscaling are production options, but they are deliberately excluded from the USD 100 sandbox.
