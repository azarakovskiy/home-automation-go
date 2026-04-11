-- name: UpsertReminder :exec
INSERT INTO reminders (id, schedule_kind, trigger_at, next_run_at, recur_every_seconds, valid_until,
    requires_ack, completion_policy, profile, status, last_fired_at, source, owner, message, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    schedule_kind = excluded.schedule_kind,
    trigger_at = excluded.trigger_at,
    next_run_at = excluded.next_run_at,
    recur_every_seconds = excluded.recur_every_seconds,
    valid_until = excluded.valid_until,
    requires_ack = excluded.requires_ack,
    completion_policy = excluded.completion_policy,
    profile = excluded.profile,
    status = excluded.status,
    last_fired_at = excluded.last_fired_at,
    source = excluded.source,
    owner = excluded.owner,
    message = excluded.message,
    updated_at = excluded.updated_at;

-- name: GetReminder :one
SELECT * FROM reminders WHERE id = ?;

-- name: ListActiveReminders :many
SELECT * FROM reminders WHERE status = 'active';

-- name: ListRemindersDueBefore :many
SELECT * FROM reminders
WHERE status = 'active'
  AND (
    (next_run_at IS NOT NULL AND next_run_at <= ?)
    OR (next_run_at IS NULL AND trigger_at <= ?)
  );

-- name: UpsertTarget :exec
INSERT OR IGNORE INTO reminder_targets (reminder_id, user_id) VALUES (?, ?);

-- name: DeleteTargets :exec
DELETE FROM reminder_targets WHERE reminder_id = ?;

-- name: ListTargets :many
SELECT user_id FROM reminder_targets WHERE reminder_id = ? ORDER BY user_id;

-- name: UpsertAck :exec
INSERT INTO reminder_acks (reminder_id, user_id, acked_at) VALUES (?, ?, ?)
ON CONFLICT(reminder_id, user_id) DO UPDATE SET acked_at = excluded.acked_at;

-- name: DeleteAcks :exec
DELETE FROM reminder_acks WHERE reminder_id = ?;

-- name: ListAcks :many
SELECT user_id, acked_at FROM reminder_acks WHERE reminder_id = ? ORDER BY user_id;
