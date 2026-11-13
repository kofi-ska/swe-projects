CREATE TABLE portfolios (
 id UUID PRIMARY KEY, name VARCHAR(120) NOT NULL UNIQUE,
 available_cash_gbp NUMERIC(19,2) NOT NULL CHECK (available_cash_gbp >= 0),
 liquidity_buffer_gbp NUMERIC(19,2) NOT NULL CHECK (liquidity_buffer_gbp >= 0 AND liquidity_buffer_gbp <= available_cash_gbp),
 configuration_version BIGINT NOT NULL CHECK (configuration_version > 0),
 created_at TIMESTAMPTZ NOT NULL, updated_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE outbox_events (
 event_id UUID PRIMARY KEY, aggregate_id UUID NOT NULL, event_type VARCHAR(100) NOT NULL,
 event_version INTEGER NOT NULL, partition_key VARCHAR(128) NOT NULL, correlation_id UUID NOT NULL,
 payload JSONB NOT NULL, occurred_at TIMESTAMPTZ NOT NULL, published_at TIMESTAMPTZ,
 attempts INTEGER NOT NULL DEFAULT 0, last_error VARCHAR(1000)
);
CREATE INDEX portfolio_outbox_unpublished_idx ON outbox_events(occurred_at,event_id) WHERE published_at IS NULL;
