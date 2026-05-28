CREATE TABLE IF NOT EXISTS decisions (
  request_id TEXT PRIMARY KEY,
  dedupe_key TEXT NOT NULL,
  decision_id TEXT NOT NULL,
  schema_version TEXT NOT NULL DEFAULT 'v1',
  terminal_state TEXT NOT NULL,
  reason_code TEXT NOT NULL,
  actionability TEXT NOT NULL,
  model_version TEXT NOT NULL,
  route_id TEXT,
  slot BIGINT NOT NULL,
  quote_age BIGINT NOT NULL,
  source_hashes TEXT[] NOT NULL,
  expected_output NUMERIC(38, 12),
  fee_cost NUMERIC(38, 12),
  slippage_cost NUMERIC(38, 12),
  breakeven_margin NUMERIC(38, 12),
  ev_estimate NUMERIC(38, 12),
  ev_lower_bound NUMERIC(38, 12),
  risk_score NUMERIC(38, 12),
  freshness_valid BOOLEAN NOT NULL DEFAULT TRUE,
  ev_realized NUMERIC(38, 12),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS decisions_dedupe_key_idx ON decisions (dedupe_key);

CREATE TABLE IF NOT EXISTS audit_outbox (
  request_id TEXT NOT NULL,
  decision_id TEXT NOT NULL,
  schema_version TEXT NOT NULL,
  trace_id TEXT NOT NULL,
  terminal_state TEXT NOT NULL,
  reason_code TEXT NOT NULL,
  model_version TEXT NOT NULL,
  route_id TEXT,
  slot BIGINT NOT NULL,
  quote_age BIGINT NOT NULL,
  source_hashes TEXT[] NOT NULL,
  stage TEXT NOT NULL,
  latency_ms BIGINT NOT NULL,
  bytes_in BIGINT NOT NULL,
  bytes_out BIGINT NOT NULL,
  success BOOLEAN NOT NULL,
  published_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (request_id, decision_id)
);

CREATE INDEX IF NOT EXISTS audit_outbox_pending_idx ON audit_outbox (published_at, created_at);
