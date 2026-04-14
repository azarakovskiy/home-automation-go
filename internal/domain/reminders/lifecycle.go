package reminders

import (
	"fmt"
	"time"
)

// New creates a validated Reminder with sensible defaults.
func New(id ReminderID, targets []string, schedule Schedule, policy DeliveryPolicy, meta Metadata, now time.Time) (Reminder, error) {
	if len(targets) == 0 {
		return Reminder{}, ErrNoTargets
	}

	switch schedule.Kind {
	case ScheduleKindOnce, ScheduleKindRecurring:
	default:
		return Reminder{}, fmt.Errorf("%w: %q", ErrInvalidSchedule, schedule.Kind)
	}

	switch policy.CompletionPolicy {
	case CompletionPolicyAllTargetsAck, CompletionPolicyAnyTargetAck:
	case "":
		policy.CompletionPolicy = CompletionPolicyAllTargetsAck
	default:
		return Reminder{}, fmt.Errorf("%w: %q", ErrInvalidPolicy, policy.CompletionPolicy)
	}

	if policy.Profile == "" {
		policy.Profile = ProfileNormal
	}

	return Reminder{
		ID:       id,
		Targets:  targets,
		Acks:     nil,
		Schedule: schedule,
		Policy:   policy,
		State: State{
			Status:    StatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Meta: meta,
	}, nil
}

// IsDue returns true if the reminder should fire at the given time.
func (r Reminder) IsDue(now time.Time) bool {
	if r.State.Status != StatusActive {
		return false
	}

	if r.Schedule.NextRunAt != nil {
		return !now.Before(*r.Schedule.NextRunAt)
	}

	return !now.Before(r.Schedule.TriggerAt)
}

// Trigger fires the reminder, advancing its state.
// For once-type reminders without ack requirement, it transitions to completed.
// For recurring reminders, it computes the next run time from RecurEvery.
// For once+requires_ack reminders, the next run time is derived from the
// escalation policy: InitialDelay on the first fire, RepeatInterval on repeats.
func (r *Reminder) Trigger(now time.Time) error {
	if r.State.Status != StatusActive {
		return ErrNotActive
	}

	isFirstFire := r.State.LastFiredAt == nil
	r.State.LastFiredAt = &now
	r.State.UpdatedAt = now

	switch {
	case r.Schedule.Kind == ScheduleKindRecurring && r.Schedule.RecurEvery != nil:
		next := now.Add(*r.Schedule.RecurEvery)
		r.Schedule.NextRunAt = &next

	case r.Schedule.Kind == ScheduleKindOnce && r.Policy.RequiresAck:
		ep := PolicyForProfile(r.Policy.Profile)
		delay := ep.RepeatInterval
		if isFirstFire {
			delay = ep.InitialDelay
		}
		next := now.Add(delay)
		r.Schedule.NextRunAt = &next

	case r.Schedule.Kind == ScheduleKindOnce && !r.Policy.RequiresAck:
		r.State.Status = StatusCompleted
	}

	return nil
}

// Acknowledge records an ack from the given user.
// It is idempotent: re-acking for the same user is a no-op.
// If the completion policy is now satisfied, the reminder transitions to completed.
func (r *Reminder) Acknowledge(userID string, now time.Time) error {
	if !r.isTarget(userID) {
		return fmt.Errorf("%w: %q", ErrNotTarget, userID)
	}

	// Idempotent: skip if already acked by this user.
	for _, a := range r.Acks {
		if a.UserID == userID {
			return nil
		}
	}

	r.Acks = append(r.Acks, UserAck{UserID: userID, AckedAt: now})
	r.State.UpdatedAt = now

	if r.IsComplete() {
		r.State.Status = StatusCompleted
	}

	return nil
}

// IsComplete checks whether the reminder's completion policy is satisfied.
func (r Reminder) IsComplete() bool {
	switch r.Policy.CompletionPolicy {
	case CompletionPolicyAnyTargetAck:
		return len(r.Acks) >= 1
	case CompletionPolicyAllTargetsAck:
		if len(r.Acks) < len(r.Targets) {
			return false
		}
		for _, t := range r.Targets {
			if !r.hasAckFrom(t) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// Delete marks the reminder as deleted.
func (r *Reminder) Delete(now time.Time) {
	r.State.Status = StatusDeleted
	r.State.UpdatedAt = now
}

// Expire marks the reminder as expired.
func (r *Reminder) Expire(now time.Time) {
	r.State.Status = StatusExpired
	r.State.UpdatedAt = now
}

// IsExpired returns true if the reminder has a ValidUntil and it has passed.
func (r Reminder) IsExpired(now time.Time) bool {
	return r.Schedule.ValidUntil != nil && now.After(*r.Schedule.ValidUntil)
}

func (r Reminder) isTarget(userID string) bool {
	for _, t := range r.Targets {
		if t == userID {
			return true
		}
	}
	return false
}

func (r Reminder) hasAckFrom(userID string) bool {
	for _, a := range r.Acks {
		if a.UserID == userID {
			return true
		}
	}
	return false
}
