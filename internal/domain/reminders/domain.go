package reminders

import "time"

type ReminderID = string

type ScheduleKind string

const (
	ScheduleKindOnce      ScheduleKind = "once"
	ScheduleKindRecurring ScheduleKind = "recurring"
)

type Profile string

const (
	ProfileQuiet    Profile = "quiet"
	ProfileNormal   Profile = "normal"
	ProfileAnnoying Profile = "annoying"
)

type Schedule struct {
	Kind       ScheduleKind
	TriggerAt  time.Time
	NextRunAt  *time.Time
	RecurEvery *time.Duration
	ValidUntil *time.Time
}

type DeliveryPolicy struct {
	RequiresAck bool
	Profile     Profile
}

type State struct {
	FireCount   int
	LastFiredAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Metadata struct {
	Source  string
	Owner   string
	Message string
}

type UserAck struct {
	UserID  string
	AckedAt time.Time
}

type Reminder struct {
	ID       ReminderID
	Targets  []string
	Acks     []UserAck
	Schedule Schedule
	Policy   DeliveryPolicy
	State    State
	Meta     Metadata
}
