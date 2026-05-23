-- Enable TimescaleDB
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- block_builds (Builder → DBPublisher)
CREATE TABLE IF NOT EXISTS block_builds (
    id SERIAL,
    block_hash TEXT NOT NULL,
    included BOOLEAN,
    inclusion_reason TEXT,
    inclusion_latency_ms BIGINT,
    timestamp TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    PRIMARY KEY (id, timestamp)
);
SELECT create_hypertable('block_builds', 'timestamp', if_not_exists => TRUE);

-- bundles (Relay → searcher submissions)
CREATE TABLE IF NOT EXISTS bundles (
    id SERIAL,
    bundle_id TEXT NOT NULL,
    searcher TEXT,
    tx_count INT,
    target_block TEXT,
    arrival_time TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    PRIMARY KEY (id, arrival_time)
);
SELECT create_hypertable('bundles', 'arrival_time', if_not_exists => TRUE);

-- simulations (Simulator → results)
CREATE TABLE IF NOT EXISTS simulations (
    id SERIAL,
    bundle_id TEXT NOT NULL,
    profit_eth DOUBLE PRECISION,
    latency_ms BIGINT,
    success BOOLEAN,
    reason TEXT,
    simulated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    PRIMARY KEY (id, simulated_at)
);
SELECT create_hypertable('simulations', 'simulated_at', if_not_exists => TRUE);

-- metrics (aggregated KPIs)
CREATE TABLE IF NOT EXISTS metrics (
    id SERIAL,
    metric_name TEXT NOT NULL,
    metric_value DOUBLE PRECISION,
    recorded_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    PRIMARY KEY (id, recorded_at)
);
SELECT create_hypertable('metrics', 'recorded_at', if_not_exists => TRUE);
