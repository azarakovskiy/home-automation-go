package reminders

import (
	"context"
	"fmt"
	"time"
)

//go:generate mockgen -destination=mocks/mock_repository.go -package=mocks home-go/internal/domain/reminders Repository
//go:generate mockgen -destination=mocks/mock_notifier.go -package=mocks home-go/internal/domain/reminders Notifier

// CreateCommand carries the parameters for creating a new reminder.
type CreateCommand struct {
	ID       ReminderID
	Targets  []string
	Schedule Schedule
	Policy   DeliveryPolicy
	Meta     Metadata
}

// Manager orchestrates reminder lifecycle operations.
type Manager struct {
	repo     Repository
	notifier Notifier
	now      func() time.Time
}

// NewManager constructs a Manager with the given repository, notifier, and clock.
// nowFn may be nil; time.Now().UTC() is used in that case.
func NewManager(repo Repository, notifier Notifier, nowFn func() time.Time) *Manager {
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	return &Manager{repo: repo, notifier: notifier, now: nowFn}
}

// Create validates and persists a new reminder.
func (m *Manager) Create(ctx context.Context, cmd CreateCommand) (Reminder, error) {
	rem, err := New(cmd.ID, cmd.Targets, cmd.Schedule, cmd.Policy, cmd.Meta, m.now())
	if err != nil {
		return Reminder{}, fmt.Errorf("build reminder: %w", err)
	}
	if err := m.repo.Save(ctx, rem); err != nil {
		return Reminder{}, fmt.Errorf("save reminder: %w", err)
	}
	return rem, nil
}

// Ack records a per-user acknowledgement. Removes the reminder if IsComplete.
func (m *Manager) Ack(ctx context.Context, reminderID ReminderID, targetUserID string) error {
	rem, err := m.repo.GetByID(ctx, reminderID)
	if err != nil {
		return fmt.Errorf("get reminder: %w", err)
	}
	if err := rem.Acknowledge(targetUserID, m.now()); err != nil {
		return fmt.Errorf("acknowledge: %w", err)
	}
	if rem.IsComplete() {
		return m.repo.Remove(ctx, reminderID)
	}
	return m.repo.Save(ctx, rem)
}

// Delete hard-deletes a reminder immediately.
func (m *Manager) Delete(ctx context.Context, reminderID ReminderID) error {
	return m.repo.Remove(ctx, reminderID)
}

// Tick evaluates all due reminders at now. Called every minute by the event handler.
//
// For each due reminder:
//  1. Hard-delete if ValidUntil has passed (no notification).
//  2. Trigger: increment FireCount, compute NextRunAt.
//  3. Hard-delete silently if RequiresAck and MaxRepeats reached (NextRunAt==nil).
//  4. Notify via Notifier.
//  5. Hard-delete if NextRunAt==nil (once/no-ack); otherwise Save.
func (m *Manager) Tick(ctx context.Context, now time.Time) error {
	due, err := m.repo.ListDueBefore(ctx, now)
	if err != nil {
		return fmt.Errorf("list due reminders: %w", err)
	}

	for i := range due {
		rem := &due[i]

		if rem.IsExpired(now) {
			if err := m.repo.Remove(ctx, rem.ID); err != nil {
				return fmt.Errorf("remove expired reminder %s: %w", rem.ID, err)
			}
			continue
		}

		rem.Trigger(now)

		// MaxRepeats reached: requires-ack reminder exhausted its fire budget silently.
		if rem.Policy.RequiresAck && rem.Schedule.NextRunAt == nil {
			if err := m.repo.Remove(ctx, rem.ID); err != nil {
				return fmt.Errorf("remove exhausted reminder %s: %w", rem.ID, err)
			}
			continue
		}

		n := Notification{ID: rem.ID, To: rem.Targets, Body: rem.Meta.Message}
		if err := m.notifier.Notify(ctx, n); err != nil {
			return fmt.Errorf("notify reminder %s: %w", rem.ID, err)
		}

		if rem.Schedule.NextRunAt == nil {
			// once/no-ack: fire-and-forget, remove after notification
			if err := m.repo.Remove(ctx, rem.ID); err != nil {
				return fmt.Errorf("remove one-shot reminder %s: %w", rem.ID, err)
			}
		} else {
			if err := m.repo.Save(ctx, *rem); err != nil {
				return fmt.Errorf("save triggered reminder %s: %w", rem.ID, err)
			}
		}
	}

	return nil
}
