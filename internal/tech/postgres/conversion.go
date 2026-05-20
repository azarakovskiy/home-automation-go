package postgres

import (
	"time"

	"home-go/internal/domain/reminders"
	"home-go/internal/tech/postgres/sqlc"
)

func rowToReminder(row sqlc.Reminder, targets []string, ackRows []sqlc.ListAcksRow) reminders.Reminder {
	sched := reminders.Schedule{
		Kind:      reminders.ScheduleKind(row.ScheduleKind),
		TriggerAt: time.Unix(row.TriggerAt, 0).UTC(),
	}
	if row.NextRunAt.Valid {
		t := time.Unix(row.NextRunAt.Int64, 0).UTC()
		sched.NextRunAt = &t
	}
	if row.RecurEverySeconds.Valid {
		d := time.Duration(row.RecurEverySeconds.Int64) * time.Second
		sched.RecurEvery = &d
	}
	if row.ValidUntil.Valid {
		t := time.Unix(row.ValidUntil.Int64, 0).UTC()
		sched.ValidUntil = &t
	}

	state := reminders.State{
		Status:    reminders.ReminderStatus(row.Status),
		CreatedAt: time.Unix(row.CreatedAt, 0).UTC(),
		UpdatedAt: time.Unix(row.UpdatedAt, 0).UTC(),
	}
	if row.LastFiredAt.Valid {
		t := time.Unix(row.LastFiredAt.Int64, 0).UTC()
		state.LastFiredAt = &t
	}

	acks := make([]reminders.UserAck, 0, len(ackRows))
	for _, a := range ackRows {
		acks = append(acks, reminders.UserAck{
			UserID:  a.UserID,
			AckedAt: time.Unix(a.AckedAt, 0).UTC(),
		})
	}

	return reminders.Reminder{
		ID:      row.ID,
		Targets: targets,
		Acks:    acks,
		Schedule: sched,
		Policy: reminders.DeliveryPolicy{
			RequiresAck:      row.RequiresAck != 0,
			CompletionPolicy: reminders.CompletionPolicy(row.CompletionPolicy),
			Profile:          reminders.Profile(row.Profile),
		},
		State: state,
		Meta: reminders.Metadata{
			Source:  row.Source,
			Owner:   row.Owner,
			Message: row.Message,
		},
	}
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
