CREATE TABLE positions (
    id UUID PRIMARY KEY,
    portfolio_id UUID NOT NULL,
    side VARCHAR(5) NOT NULL CHECK (side IN ('LONG', 'SHORT')),
    contract_price NUMERIC(19, 6) NOT NULL,
    volume_mwh NUMERIC(19, 6) NOT NULL CHECK (volume_mwh > 0 AND volume_mwh <= 1000000),
    settlement_date DATE NOT NULL,
    settlement_period SMALLINT NOT NULL CHECK (settlement_period BETWEEN 1 AND 50),
    delivery_start TIMESTAMPTZ NOT NULL,
    delivery_end TIMESTAMPTZ NOT NULL CHECK (delivery_end > delivery_start),
    status VARCHAR(16) NOT NULL CHECK (status IN ('ACTIVE', 'SETTLED')),
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX positions_portfolio_delivery_idx ON positions (portfolio_id, delivery_start, id);
CREATE INDEX positions_status_delivery_idx ON positions (status, delivery_start, id);

CREATE TABLE idempotency_records (
    idempotency_key VARCHAR(128) PRIMARY KEY,
    request_hash CHAR(64) NOT NULL,
    position_id UUID NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT idempotency_position_fk FOREIGN KEY (position_id) REFERENCES positions(id) DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE outbox_events (
    event_id UUID PRIMARY KEY,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_version INTEGER NOT NULL CHECK (event_version > 0),
    partition_key VARCHAR(128) NOT NULL,
    correlation_id UUID NOT NULL,
    causation_id UUID,
    payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error VARCHAR(1000)
);

CREATE INDEX outbox_unpublished_idx ON outbox_events (occurred_at, event_id) WHERE published_at IS NULL;
