package reminders

import "time"

// ReminderID is a unique identifier for a reminder.
type ReminderID = string

// ScheduleKind defines whether a reminder fires once or recurs.
type ScheduleKind string

const (
	ScheduleKindOnce      ScheduleKind = "once"
	ScheduleKindRecurring ScheduleKind = "recurring"
)

// CompletionPolicy defines when a reminder is considered fully acknowledged.
type CompletionPolicy string

const (
	CompletionPolicyAllTargetsAck CompletionPolicy = "all_targets_ack"
	CompletionPolicyAnyTargetAck  CompletionPolicy = "any_target_ack"
)

// Profile controls escalation behavior.
type Profile string

const (
	ProfileQuiet    Profile = "quiet"
	ProfileNormal   Profile = "normal"
	ProfileAnnoying Profile = "annoying"
)

// ReminderStatus represents the lifecycle state of a reminder.
type ReminderStatus string

const (
	StatusActive    ReminderStatus = "active"
	StatusCompleted ReminderStatus = "completed"
	StatusDeleted   ReminderStatus = "deleted"
	StatusExpired   ReminderStatus = "expired"
)

// Schedule defines when and how often a reminder fires.
type Schedule struct {
	Kind       ScheduleKind
	TriggerAt  time.Time
	NextRunAt  *time.Time     // nil on first run; set after each trigger for recurring
	RecurEvery *time.Duration // nil for once
	ValidUntil *time.Time     // nil means no expiry
}

// DeliveryPolicy controls acknowledgement and escalation behavior.
type DeliveryPolicy struct {
	RequiresAck      bool
	CompletionPolicy CompletionPolicy
	Profile          Profile
}

// State holds the runtime lifecycle state of a reminder.
type State struct {
	Status      ReminderStatus
	LastFiredAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Metadata stores informational fields about a reminder.
type Metadata struct {
	Source  string
	Owner   string
	Message string // human-readable reminder text
}

// UserAck records a single user's acknowledgement.
type UserAck struct {
	UserID  string
	AckedAt time.Time
}

// Reminder is the root aggregate for the reminders domain.
type Reminder struct {
	ID       ReminderID
	Targets  []string  // user IDs, 1+
	Acks     []UserAck // per-user ack records
	Schedule Schedule
	Policy   DeliveryPolicy
	State    State
	Meta     Metadata
}
