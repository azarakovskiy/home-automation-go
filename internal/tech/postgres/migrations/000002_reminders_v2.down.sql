ALTER TABLE reminders
    ADD COLUMN status TEXT NOT NULL DEFAULT 'active',
    ADD COLUMN completion_policy TEXT NOT NULL DEFAULT 'all_targets_ack';

ALTER TABLE reminders
    DROP COLUMN requires_ack;

ALTER TABLE reminders
    ADD COLUMN requires_ack BIGINT NOT NULL DEFAULT 0;

ALTER TABLE reminders
    DROP COLUMN fire_count;

CREATE INDEX IF NOT EXISTS idx_reminders_status ON reminders(status);
