CREATE TABLE IF NOT EXISTS upstream_resource_accounts (
    id BIGSERIAL PRIMARY KEY,
    resource_id BIGINT NOT NULL REFERENCES upstream_resources(id) ON DELETE CASCADE,
    platform VARCHAR(50) NOT NULL,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    rate_multiplier DECIMAL(10,4) NOT NULL,
    synced_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT upstream_resource_accounts_resource_platform_unique UNIQUE (resource_id, platform),
    CONSTRAINT upstream_resource_accounts_account_unique UNIQUE (account_id)
);

INSERT INTO upstream_resource_accounts (resource_id, platform, account_id, rate_multiplier, synced_at)
SELECT r.id, a.platform, a.id, COALESCE(r.synced_rate_multiplier, 1), COALESCE(r.synced_at, r.updated_at)
FROM upstream_resources r
JOIN accounts a ON a.id = r.synced_account_id
WHERE r.synced_account_id IS NOT NULL
ON CONFLICT DO NOTHING;
