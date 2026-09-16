ALTER TABLE upstreams
    ADD COLUMN IF NOT EXISTS password_encrypted TEXT NOT NULL DEFAULT '';
