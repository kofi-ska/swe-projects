CREATE TABLE IF NOT EXISTS decisions (
  request_id TEXT PRIMARY KEY,
  dedupe_key TEXT NOT NULL,
  decision_id TEXT NOT NULL,
  terminal_state TEXT NOT NULL,
  reason_code TEXT NOT NULL,
  model_version TEXT NOT NULL,
  route_id TEXT,
  slot BIGINT NOT NULL,
  quote_age BIGINT NOT NULL,
  source_hashes TEXT NOT NULL,
  ev_estimate TEXT,
  ev_lower_bound TEXT,
  ev_realized TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS decisions_dedupe_key_idx ON decisions (dedupe_key);
