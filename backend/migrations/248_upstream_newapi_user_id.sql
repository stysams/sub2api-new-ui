ALTER TABLE upstreams
    ADD COLUMN IF NOT EXISTS remote_user_id VARCHAR(100);
