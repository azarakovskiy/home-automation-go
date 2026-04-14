DROP INDEX IF EXISTS idx_reminders_trigger_at;
DROP INDEX IF EXISTS idx_reminders_next_run_at;
DROP INDEX IF EXISTS idx_reminders_status;
DROP TABLE IF EXISTS reminder_acks;
DROP TABLE IF EXISTS reminder_targets;
DROP TABLE IF EXISTS reminders;
