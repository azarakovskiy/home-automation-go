package reminders

import (
	"context"
	"time"
)

// Repository defines persistence operations for reminders.
type Repository interface {
	Save(ctx context.Context, r Reminder) error
	GetByID(ctx context.Context, id ReminderID) (Reminder, error)
	ListActive(ctx context.Context) ([]Reminder, error)
	ListDueBefore(ctx context.Context, t time.Time) ([]Reminder, error)
	Delete(ctx context.Context, id ReminderID) error
}
