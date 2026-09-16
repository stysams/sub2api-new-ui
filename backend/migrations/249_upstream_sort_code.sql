ALTER TABLE upstreams
    ADD COLUMN IF NOT EXISTS sort_code INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_upstreams_sort_code
    ON upstreams (sort_code, id);
