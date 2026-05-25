ALTER TABLE reminders
    ADD COLUMN fire_count INTEGER NOT NULL DEFAULT 0;

ALTER TABLE reminders
    DROP COLUMN requires_ack;

ALTER TABLE reminders
    ADD COLUMN requires_ack BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE reminders
    DROP COLUMN status,
    DROP COLUMN completion_policy;

DROP INDEX IF EXISTS idx_reminders_status;
