CREATE TABLE IF NOT EXISTS reminders (
    id TEXT PRIMARY KEY,
    schedule_kind TEXT NOT NULL CHECK (schedule_kind IN ('once', 'recurring')),
    trigger_at BIGINT NOT NULL,
    next_run_at BIGINT,
    recur_every_seconds BIGINT,
    valid_until BIGINT,
    requires_ack BOOLEAN NOT NULL DEFAULT false,
    fire_count INTEGER NOT NULL DEFAULT 0,
    profile TEXT NOT NULL DEFAULT 'normal',
    last_fired_at BIGINT,
    source TEXT NOT NULL DEFAULT '',
    owner TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS reminder_targets (
    reminder_id TEXT NOT NULL REFERENCES reminders(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    PRIMARY KEY (reminder_id, user_id)
);

CREATE TABLE IF NOT EXISTS reminder_acks (
    reminder_id TEXT NOT NULL REFERENCES reminders(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    acked_at BIGINT NOT NULL,
    PRIMARY KEY (reminder_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_reminders_next_run_at ON reminders(next_run_at);
CREATE INDEX IF NOT EXISTS idx_reminders_trigger_at ON reminders(trigger_at);
