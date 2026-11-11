-- Independent evidence/history tables; no account schema or routing changes.
-- No account FK: preserve evidence when an upstream account is removed.
CREATE TABLE IF NOT EXISTS model_fingerprint_records (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL,
    account_name TEXT NOT NULL DEFAULT '',
    platform TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL,
    upstream_model TEXT NOT NULL DEFAULT '',
    endpoint TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    score INTEGER NOT NULL DEFAULT 0,
    samples JSONB NOT NULL DEFAULT '[]'::jsonb,
    analysis JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_model_fingerprint_records_created
    ON model_fingerprint_records (created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_model_fingerprint_records_account_model
    ON model_fingerprint_records (account_id, model, created_at DESC, id DESC);
CREATE TABLE IF NOT EXISTS model_fingerprint_reference (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    bank JSONB NOT NULL,
    detector BYTEA NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
