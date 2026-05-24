# v2 Docs Index

Operator notes for v2.

## What lives here

- `architecture.md` - system shape and ownership
- `refactor-proposal.md` - what changed and why
- `observability.md` - metrics, traces, logs, dashboards
- `alerting.md` - alert policy and severity
- `alerting-rules.yaml` - sample Prometheus rules
- `../infra/prometheus.yml` - scrape and rule wiring
- `../infra/alertmanager.yml` - local alert routing
- `runbooks.md` - incident playbooks
- `recovery.md` - restart and replay procedure
- `performance.md` - budgets and limits
- `lifecycle-math.md` - quantified lifecycle model
- `security.md` - trust boundaries and minimum hardening
- `testing.md` - test coverage we want to keep
- `failure-taxonomy.md` - stable error classes

## Intent

Keep the code path small. Keep the docs usable during an incident.
