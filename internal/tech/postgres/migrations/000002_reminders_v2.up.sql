ALTER TABLE reminders
    ADD COLUMN fire_count INTEGER NOT NULL DEFAULT 0;

ALTER TABLE reminders
    ALTER COLUMN requires_ack TYPE BOOLEAN USING (requires_ack != 0);

ALTER TABLE reminders
    DROP COLUMN status,
    DROP COLUMN completion_policy;

DROP INDEX IF EXISTS idx_reminders_status;
