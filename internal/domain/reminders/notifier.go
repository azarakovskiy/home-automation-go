package reminders

import "context"

// Notification is the payload sent to downstream delivery channels.
type Notification struct {
	ID   ReminderID
	To   []string
	Body string
}

// Notifier delivers a notification to one or more targets.
type Notifier interface {
	Notify(ctx context.Context, n Notification) error
}
