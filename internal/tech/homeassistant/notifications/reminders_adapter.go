package notifications

import (
	"context"
	"fmt"

	domainreminders "home-go/internal/domain/reminders"
)

// ReminderNotifier wraps NotificationService to satisfy reminders.Notifier.
type ReminderNotifier struct {
	svc *NotificationService
}

// NewReminderNotifier creates a ReminderNotifier backed by svc.
func NewReminderNotifier(svc *NotificationService) *ReminderNotifier {
	return &ReminderNotifier{svc: svc}
}

func (a *ReminderNotifier) Notify(_ context.Context, n domainreminders.Notification) error {
	for _, userID := range n.To {
		if err := a.svc.Notify(Event{
			Device:  userID,
			Type:    "reminder",
			Message: n.Body,
		}); err != nil {
			return fmt.Errorf("notify %s: %w", userID, err)
		}
	}
	return nil
}
