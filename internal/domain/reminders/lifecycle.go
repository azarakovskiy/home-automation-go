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
			CreatedAt: now,
			UpdatedAt: now,
		},
		Meta: meta,
	}, nil
}

// IsDue returns true if the reminder should fire at the given time.
func (r Reminder) IsDue(now time.Time) bool {
	if r.Schedule.NextRunAt != nil {
		return !now.Before(*r.Schedule.NextRunAt)
	}
	return !now.Before(r.Schedule.TriggerAt)
}

// Trigger fires the reminder, advancing FireCount and computing the next run time.
// For once+requires_ack reminders: uses escalation policy; clears NextRunAt when MaxRepeats reached.
// For recurring reminders: advances by RecurEvery.
// For once+no-ack reminders: clears NextRunAt (fire-and-forget).
func (r *Reminder) Trigger(now time.Time) {
	r.State.FireCount++
	r.State.LastFiredAt = &now
	r.State.UpdatedAt = now

	switch {
	case r.Schedule.Kind == ScheduleKindRecurring:
		next := now.Add(*r.Schedule.RecurEvery)
		r.Schedule.NextRunAt = &next

	case r.Policy.RequiresAck:
		ep := PolicyForProfile(r.Policy.Profile)
		if ep.MaxRepeats > 0 && r.State.FireCount >= ep.MaxRepeats {
			r.Schedule.NextRunAt = nil
			return
		}
		delay := r.repeatDelay(ep)
		next := now.Add(delay)
		r.Schedule.NextRunAt = &next

	default:
		// once, no ack required — caller will remove after notification
		r.Schedule.NextRunAt = nil
	}
}

// repeatDelay computes the delay to the next fire based on current FireCount and policy.
func (r Reminder) repeatDelay(ep EscalationPolicy) time.Duration {
	if r.State.FireCount == 1 {
		return ep.InitialDelay
	}
	if ep.DecreaseStep > 0 {
		reduced := ep.RepeatInterval - time.Duration(r.State.FireCount-2)*ep.DecreaseStep
		if reduced < ep.MinInterval {
			return ep.MinInterval
		}
		return reduced
	}
	return ep.RepeatInterval
}

// Acknowledge records an ack for targetUserID. Idempotent for the same user.
func (r *Reminder) Acknowledge(targetUserID string, now time.Time) error {
	if !r.isTarget(targetUserID) {
		return fmt.Errorf("%w: %q", ErrNotTarget, targetUserID)
	}
	for _, a := range r.Acks {
		if a.UserID == targetUserID {
			return nil
		}
	}
	r.Acks = append(r.Acks, UserAck{UserID: targetUserID, AckedAt: now})
	r.State.UpdatedAt = now
	return nil
}

// IsComplete returns true if any target has acknowledged the reminder.
func (r Reminder) IsComplete() bool {
	return len(r.Acks) >= 1
}

// IsExpired returns true if ValidUntil is set and has passed.
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
