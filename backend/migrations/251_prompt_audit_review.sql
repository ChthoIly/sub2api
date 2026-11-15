-- Preserve the scanner's original decision while allowing admins to attach a
-- manual review outcome. Lists, filters, and aggregate views use the effective
-- decision, while the raw scanner result remains immutable for audit purposes.
ALTER TABLE prompt_audit_events
    ADD COLUMN IF NOT EXISTS review_status VARCHAR(32) NOT NULL DEFAULT 'unreviewed',
    ADD COLUMN IF NOT EXISTS review_note VARCHAR(512) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMPTZ;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_prompt_audit_events_review_status'
    ) THEN
        ALTER TABLE prompt_audit_events
            ADD CONSTRAINT chk_prompt_audit_events_review_status
            CHECK (review_status IN ('unreviewed', 'confirmed_false_positive'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_prompt_audit_events_review_status_created
    ON prompt_audit_events (review_status, created_at DESC, id DESC);
