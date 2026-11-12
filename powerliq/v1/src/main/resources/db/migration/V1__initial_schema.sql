CREATE TABLE portfolios (
    id UUID PRIMARY KEY,
    currency CHAR(3) NOT NULL CHECK (currency = 'GBP'),
    available_cash NUMERIC(19, 2) NOT NULL CHECK (available_cash >= 0),
    liquidity_buffer NUMERIC(19, 2) NOT NULL CHECK (liquidity_buffer >= 0)
);

INSERT INTO portfolios (id, currency, available_cash, liquidity_buffer)
VALUES ('10000000-0000-0000-0000-000000000001', 'GBP', 250000.00, 100000.00);

CREATE TABLE positions (
    id UUID PRIMARY KEY,
    portfolio_id UUID NOT NULL REFERENCES portfolios(id),
    side VARCHAR(5) NOT NULL CHECK (side IN ('LONG', 'SHORT')),
    contract_price NUMERIC(19, 5) NOT NULL,
    volume_mwh NUMERIC(19, 3) NOT NULL CHECK (volume_mwh > 0 AND volume_mwh <= 1000000),
    settlement_date DATE NOT NULL,
    settlement_period SMALLINT NOT NULL CHECK (settlement_period > 0),
    delivery_start TIMESTAMPTZ NOT NULL,
    delivery_end TIMESTAMPTZ NOT NULL,
    status VARCHAR(7) NOT NULL CHECK (status IN ('ACTIVE', 'SETTLED')),
    created_at TIMESTAMPTZ NOT NULL,
    CHECK (delivery_end = delivery_start + INTERVAL '30 minutes')
);

CREATE INDEX positions_portfolio_status_idx ON positions (portfolio_id, status);

CREATE TABLE idempotency_records (
    idempotency_key VARCHAR(128) PRIMARY KEY,
    request_hash CHAR(64) NOT NULL,
    position_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT idempotency_position_fk FOREIGN KEY (position_id) REFERENCES positions(id)
        DEFERRABLE INITIALLY DEFERRED
);

CREATE TABLE market_prices (
    provider VARCHAR(32) NOT NULL,
    settlement_date DATE NOT NULL,
    settlement_period SMALLINT NOT NULL CHECK (settlement_period > 0),
    delivery_start TIMESTAMPTZ NOT NULL,
    delivery_end TIMESTAMPTZ NOT NULL,
    price NUMERIC(19, 5) NOT NULL,
    source_timestamp TIMESTAMPTZ,
    retrieved_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (provider, settlement_date, settlement_period),
    CHECK (delivery_end = delivery_start + INTERVAL '30 minutes')
);
