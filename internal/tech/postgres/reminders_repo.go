package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"home-go/internal/domain/reminders"
	"home-go/internal/tech/postgres/sqlc"
)

// Conversion helpers live in conversion.go.

// RemindersRepo implements reminders.Repository using PostgreSQL via sqlc.
type RemindersRepo struct {
	db      *sql.DB
	queries *sqlc.Queries
}

// NewRemindersRepo constructs a RemindersRepo backed by the given *sql.DB.
func NewRemindersRepo(db *sql.DB) *RemindersRepo {
	return &RemindersRepo{
		db:      db,
		queries: sqlc.New(db),
	}
}

// Save persists a Reminder aggregate (upsert + full replacement of targets and acks).
func (r *RemindersRepo) Save(ctx context.Context, rem reminders.Reminder) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	q := r.queries.WithTx(tx)

	params := sqlc.UpsertReminderParams{
		ID:           rem.ID,
		ScheduleKind: string(rem.Schedule.Kind),
		TriggerAt:    rem.Schedule.TriggerAt.Unix(),
		RequiresAck:  rem.Policy.RequiresAck,
		Profile:      string(rem.Policy.Profile),
		FireCount:    int32(rem.State.FireCount),
		Source:       rem.Meta.Source,
		Owner:        rem.Meta.Owner,
		Message:      rem.Meta.Message,
		CreatedAt:    rem.State.CreatedAt.Unix(),
		UpdatedAt:    rem.State.UpdatedAt.Unix(),
	}

	if rem.Schedule.NextRunAt != nil {
		params.NextRunAt = sql.NullInt64{Int64: rem.Schedule.NextRunAt.Unix(), Valid: true}
	}
	if rem.Schedule.RecurEvery != nil {
		secs := int64(rem.Schedule.RecurEvery.Seconds())
		params.RecurEverySeconds = sql.NullInt64{Int64: secs, Valid: true}
	}
	if rem.Schedule.ValidUntil != nil {
		params.ValidUntil = sql.NullInt64{Int64: rem.Schedule.ValidUntil.Unix(), Valid: true}
	}
	if rem.State.LastFiredAt != nil {
		params.LastFiredAt = sql.NullInt64{Int64: rem.State.LastFiredAt.Unix(), Valid: true}
	}

	if err := q.UpsertReminder(ctx, params); err != nil {
		return fmt.Errorf("upsert reminder: %w", err)
	}

	if err := q.DeleteTargets(ctx, rem.ID); err != nil {
		return fmt.Errorf("delete targets: %w", err)
	}
	for _, userID := range rem.Targets {
		if err := q.UpsertTarget(ctx, sqlc.UpsertTargetParams{
			ReminderID: rem.ID,
			UserID:     userID,
		}); err != nil {
			return fmt.Errorf("upsert target %s: %w", userID, err)
		}
	}

	if err := q.DeleteAcks(ctx, rem.ID); err != nil {
		return fmt.Errorf("delete acks: %w", err)
	}
	for _, ack := range rem.Acks {
		if err := q.UpsertAck(ctx, sqlc.UpsertAckParams{
			ReminderID: rem.ID,
			UserID:     ack.UserID,
			AckedAt:    ack.AckedAt.Unix(),
		}); err != nil {
			return fmt.Errorf("upsert ack %s: %w", ack.UserID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// GetByID loads a full Reminder aggregate by ID.
func (r *RemindersRepo) GetByID(ctx context.Context, id reminders.ReminderID) (reminders.Reminder, error) {
	row, err := r.queries.GetReminder(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return reminders.Reminder{}, reminders.ErrNotFound
		}
		return reminders.Reminder{}, fmt.Errorf("get reminder: %w", err)
	}
	return r.loadAggregate(ctx, row)
}

// ListActive returns all stored reminders (all stored reminders are active).
func (r *RemindersRepo) ListActive(ctx context.Context) ([]reminders.Reminder, error) {
	rows, err := r.queries.ListActiveReminders(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active reminders: %w", err)
	}
	return r.hydrateList(ctx, rows)
}

// ListDueBefore returns active reminders due at or before t.
func (r *RemindersRepo) ListDueBefore(ctx context.Context, t time.Time) ([]reminders.Reminder, error) {
	unix := t.Unix()
	params := sqlc.ListRemindersDueBeforeParams{
		NextRunAt: sql.NullInt64{Int64: unix, Valid: true},
		TriggerAt: unix,
	}
	rows, err := r.queries.ListRemindersDueBefore(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list reminders due before: %w", err)
	}
	return r.hydrateList(ctx, rows)
}

// Remove hard-deletes a reminder by ID.
func (r *RemindersRepo) Remove(ctx context.Context, id reminders.ReminderID) error {
	if err := r.queries.DeleteReminder(ctx, id); err != nil {
		return fmt.Errorf("delete reminder %s: %w", id, err)
	}
	return nil
}

func (r *RemindersRepo) loadAggregate(ctx context.Context, row sqlc.Reminder) (reminders.Reminder, error) {
	targets, err := r.queries.ListTargets(ctx, row.ID)
	if err != nil {
		return reminders.Reminder{}, fmt.Errorf("list targets for %s: %w", row.ID, err)
	}
	ackRows, err := r.queries.ListAcks(ctx, row.ID)
	if err != nil {
		return reminders.Reminder{}, fmt.Errorf("list acks for %s: %w", row.ID, err)
	}
	return rowToReminder(row, targets, ackRows), nil
}

func (r *RemindersRepo) hydrateList(ctx context.Context, rows []sqlc.Reminder) ([]reminders.Reminder, error) {
	out := make([]reminders.Reminder, 0, len(rows))
	for _, row := range rows {
		rem, err := r.loadAggregate(ctx, row)
		if err != nil {
			return nil, fmt.Errorf("hydrate reminder %s: %w", row.ID, err)
		}
		out = append(out, rem)
	}
	return out, nil
}
