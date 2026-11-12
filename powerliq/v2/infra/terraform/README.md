# PowerLiq sandbox infrastructure

This stack deploys five independently versioned ECS services onto one Graviton ECS instance, a single encrypted PostgreSQL RDS instance with service-owned schemas, an HTTPS ALB, ECR, Cognito, encrypted CloudWatch logs, alarms, dashboard, SNS notifications and an AWS Budget.

The default planning estimate is **USD 74/month** in `eu-west-2`: ECS EC2 `t4g.medium` ~USD 25, RDS `db.t4g.micro` and 20 GB ~USD 16, ALB ~USD 18, storage/logs/metrics/ECR/data allowance ~USD 15. Prices vary; this is not a billing guarantee. Terraform rejects a configured budget over USD 100 and rejects larger compute/database classes. AWS Budget alerts at forecast 80% and actual 100%.

There is deliberately no NAT Gateway, Kinesis, managed Kafka, multi-AZ RDS, Performance Insights or unbounded log retention in the sandbox. Those improve resilience or throughput but violate this cost envelope.

Run `terraform init`, `terraform plan -var-file=terraform.tfvars`, then review the plan and estimate before applying. Supply an ACM certificate and immutable image tag. The SNS email subscription must be confirmed manually.

RTO/RPO assumptions for this sandbox are not production guarantees: daily RDS backups imply RPO up to 24 hours; single-AZ instance replacement targets RTO under four hours. Measure recovery before changing these from objectives to confirmed SLOs.
