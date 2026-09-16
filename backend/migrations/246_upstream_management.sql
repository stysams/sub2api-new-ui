CREATE TABLE IF NOT EXISTS upstreams (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    kind VARCHAR(20) NOT NULL,
    base_url VARCHAR(500) NOT NULL,
    token_encrypted TEXT NOT NULL DEFAULT '',
    refresh_token_encrypted TEXT,
    token_expires_at TIMESTAMPTZ,
    login_identifier VARCHAR(200),
    balance_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    group_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    notes TEXT,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_checked_at TIMESTAMPTZ,
    last_error TEXT,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_upstreams_kind ON upstreams (kind);
CREATE INDEX IF NOT EXISTS idx_upstreams_enabled ON upstreams (enabled);
CREATE INDEX IF NOT EXISTS idx_upstreams_last_checked_at ON upstreams (last_checked_at);

CREATE TABLE IF NOT EXISTS upstream_resources (
    id BIGSERIAL PRIMARY KEY,
    upstream_id BIGINT NOT NULL,
    resource_type VARCHAR(20) NOT NULL,
    remote_id VARCHAR(100) NOT NULL,
    name VARCHAR(200) NOT NULL DEFAULT '',
    group_name VARCHAR(200),
    key_encrypted TEXT NOT NULL DEFAULT '',
    models_snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,
    models_fetched_at TIMESTAMPTZ,
    synced_account_id BIGINT,
    synced_rate_multiplier DECIMAL(10,4),
    synced_at TIMESTAMPTZ,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT upstream_resources_identity_unique UNIQUE (upstream_id, resource_type, remote_id)
);

CREATE INDEX IF NOT EXISTS idx_upstream_resources_upstream_id ON upstream_resources (upstream_id);
CREATE INDEX IF NOT EXISTS idx_upstream_resources_upstream_type ON upstream_resources (upstream_id, resource_type);
CREATE INDEX IF NOT EXISTS idx_upstream_resources_synced_account_id ON upstream_resources (synced_account_id);
