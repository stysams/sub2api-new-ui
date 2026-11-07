ALTER TABLE prompt_records
    ADD COLUMN IF NOT EXISTS recording_policy TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS response_document TEXT NOT NULL DEFAULT '';
