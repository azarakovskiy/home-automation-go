package reminders

import (
	"context"
	"fmt"
	"time"
)

//go:generate mockgen -destination=mocks/mock_repository.go -package=mocks home-go/internal/domain/reminders Repository

// ActionKind describes what the adapter layer should do after a manager operation.
type ActionKind string

const (
	// ActionShowProjection instructs the adapter to create or refresh the
	// reminder projection for all target users.
	ActionShowProjection ActionKind = "show"

	// ActionRemoveProjection instructs the adapter to tear down the reminder
	// projection for all target users.
	ActionRemoveProjection ActionKind = "remove"

	// ActionNoop means no projection change is needed.
	ActionNoop ActionKind = "noop"
)

// Action is the result of a manager operation that the adapter layer acts on.
type Action struct {
	Kind     ActionKind
	Reminder Reminder // populated for show/refresh; zero value for remove and noop
}

// Manager orchestrates reminder lifecycle operations against a Repository.
// It is the single entry point for mutating reminder state; all callers
// (adapters, tick scheduler) go through here.
type Manager struct {
	repo Repository
	now  func() time.Time // injectable for tests
}

// NewManager constructs a Manager backed by the given Repository.
// nowFn may be nil; time.Now().UTC() is used in that case.
func NewManager(repo Repository, nowFn func() time.Time) *Manager {
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}
	return &Manager{repo: repo, now: nowFn}
}

// Create validates and persists a new reminder, returning a ShowProjection action.
func (m *Manager) Create(ctx context.Context, id ReminderID, targets []string, schedule Schedule, policy DeliveryPolicy, meta Metadata) (Action, error) {
	rem, err := New(id, targets, schedule, policy, meta, m.now())
	if err != nil {
		return Action{}, fmt.Errorf("build reminder: %w", err)
	}

	if err := m.repo.Save(ctx, rem); err != nil {
		return Action{}, fmt.Errorf("save reminder: %w", err)
	}

	return Action{Kind: ActionShowProjection, Reminder: rem}, nil
}

// Ack records a per-user acknowledgement.
// Returns RemoveProjection if the reminder is now complete, ShowProjection otherwise.
func (m *Manager) Ack(ctx context.Context, reminderID ReminderID, userID string) (Action, error) {
	rem, err := m.repo.GetByID(ctx, reminderID)
	if err != nil {
		return Action{}, fmt.Errorf("get reminder: %w", err)
	}

	if err := rem.Acknowledge(userID, m.now()); err != nil {
		return Action{}, fmt.Errorf("acknowledge: %w", err)
	}

	if err := m.repo.Save(ctx, rem); err != nil {
		return Action{}, fmt.Errorf("save reminder after ack: %w", err)
	}

	if rem.State.Status == StatusCompleted {
		return Action{Kind: ActionRemoveProjection, Reminder: rem}, nil
	}
	return Action{Kind: ActionShowProjection, Reminder: rem}, nil
}

// Delete marks a reminder as deleted and returns a RemoveProjection action.
func (m *Manager) Delete(ctx context.Context, reminderID ReminderID) (Action, error) {
	rem, err := m.repo.GetByID(ctx, reminderID)
	if err != nil {
		return Action{}, fmt.Errorf("get reminder: %w", err)
	}

	rem.Delete(m.now())

	if err := m.repo.Save(ctx, rem); err != nil {
		return Action{}, fmt.Errorf("save reminder after delete: %w", err)
	}

	return Action{Kind: ActionRemoveProjection, Reminder: rem}, nil
}

// Restore returns all active reminders so the adapter can rebuild projections
// on startup without re-triggering them.
func (m *Manager) Restore(ctx context.Context) ([]Reminder, error) {
	list, err := m.repo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active reminders: %w", err)
	}
	return list, nil
}

// Tick evaluates all due and expired reminders at the given instant.
// It returns one Action per affected reminder; callers should process them all.
func (m *Manager) Tick(ctx context.Context, now time.Time) ([]Action, error) {
	due, err := m.repo.ListDueBefore(ctx, now)
	if err != nil {
		return nil, fmt.Errorf("list due reminders: %w", err)
	}

	var actions []Action

	for i := range due {
		rem := &due[i]

		// Expiry takes priority over triggering.
		if rem.IsExpired(now) {
			rem.Expire(now)
			if err := m.repo.Save(ctx, *rem); err != nil {
				return nil, fmt.Errorf("save expired reminder %s: %w", rem.ID, err)
			}
			actions = append(actions, Action{Kind: ActionRemoveProjection, Reminder: *rem})
			continue
		}

		if err := rem.Trigger(now); err != nil {
			return nil, fmt.Errorf("trigger reminder %s: %w", rem.ID, err)
		}

		if err := m.repo.Save(ctx, *rem); err != nil {
			return nil, fmt.Errorf("save triggered reminder %s: %w", rem.ID, err)
		}

		if rem.State.Status == StatusCompleted {
			actions = append(actions, Action{Kind: ActionRemoveProjection, Reminder: *rem})
		} else {
			actions = append(actions, Action{Kind: ActionShowProjection, Reminder: *rem})
		}
	}

	return actions, nil
}
