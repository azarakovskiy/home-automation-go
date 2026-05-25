package reminders

import (
	"context"
	"time"
)

// Repository defines persistence operations for reminders.
type Repository interface {
	Save(ctx context.Context, r Reminder) error
	GetByID(ctx context.Context, id ReminderID) (Reminder, error)
	List(ctx context.Context) ([]Reminder, error)
	ListDueBefore(ctx context.Context, t time.Time) ([]Reminder, error)
	Remove(ctx context.Context, id ReminderID) error
}
