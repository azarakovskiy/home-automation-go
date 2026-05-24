ALTER TABLE reminders
    ADD COLUMN status TEXT NOT NULL DEFAULT 'active',
    ADD COLUMN completion_policy TEXT NOT NULL DEFAULT 'all_targets_ack';

ALTER TABLE reminders
    ALTER COLUMN requires_ack TYPE BIGINT USING (CASE WHEN requires_ack THEN 1 ELSE 0 END);

ALTER TABLE reminders
    DROP COLUMN fire_count;

CREATE INDEX IF NOT EXISTS idx_reminders_status ON reminders(status);
