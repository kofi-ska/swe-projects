CREATE TABLE valuations(id UUID PRIMARY KEY,idempotency_key VARCHAR(128) NOT NULL UNIQUE,input_hash CHAR(64) NOT NULL,portfolio_id UUID NOT NULL,as_of TIMESTAMPTZ NOT NULL,calculated_at TIMESTAMPTZ NOT NULL,input_json JSONB NOT NULL,result_json JSONB NOT NULL);
CREATE INDEX valuations_portfolio_asof_idx ON valuations(portfolio_id,as_of DESC,id);
