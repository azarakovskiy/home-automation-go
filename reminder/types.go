package reminder

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ReminderProfile defines how the reminder repeats.
type ReminderProfile string

const (
	ProfileNormal   ReminderProfile = "normal"
	ProfileAnnoying ReminderProfile = "annoying"
	ProfileQuiet    ReminderProfile = "quiet"
)

// ReminderMode describes whether the reminder repeats or is a one-time notification.
type ReminderMode string

const (
	ModeRepeating ReminderMode = "repeating"
	ModeSingle    ReminderMode = "single"
)

const (
	DefaultInitialRepeatMinutes = 20
	DefaultMinRepeatMinutes     = 2
	DefaultMaxRepeatMinutes     = 180 // 3 hours

	annoyingDecayFactor = 0.5
	quietGrowthFactor   = 1.5
)

// QuietHoursConfig specifies when reminders should pause.
type QuietHoursConfig struct {
	Enabled bool   `json:"enabled"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

// ReminderDefinition stores persisted configuration.
type ReminderDefinition struct {
	ID               string            `json:"id"`
	Title            string            `json:"title"`
	Message          string            `json:"message"`
	Profile          ReminderProfile   `json:"profile"`
	Mode             ReminderMode      `json:"mode"`
	StartTime        time.Time         `json:"start_time"`
	InitialRepeatMin int               `json:"initial_repeat_minutes"`
	MinRepeatMin     int               `json:"min_repeat_minutes"`
	MaxRepeatMin     int               `json:"max_repeat_minutes"`
	SpeakerEntity    string            `json:"speaker_entity,omitempty"`
	PhoneNotifier    string            `json:"phone_notifier,omitempty"`
	VisibleTo        []string          `json:"visible_to,omitempty"`
	NightModeAllowed bool              `json:"night_mode_allowed"`
	PresenceRequired bool              `json:"presence_required"`
	QuietHours       QuietHoursConfig  `json:"quiet_hours"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
}

// ReminderRuntime tracks scheduling state.
type ReminderRuntime struct {
	ID              string    `json:"id"`
	NextTrigger     time.Time `json:"next_trigger"`
	RepeatCount     int       `json:"repeat_count"`
	LastIntervalMin int       `json:"last_interval_minutes"`
	LastTriggered   time.Time `json:"last_triggered"`
	Completed       bool      `json:"completed"`
	Cancelled       bool      `json:"cancelled"`
	AcknowledgedBy  string    `json:"acknowledged_by,omitempty"`
	AcknowledgedAt  time.Time `json:"acknowledged_at,omitempty"`
	LastFailure     string    `json:"last_failure,omitempty"`
	LastFailureTime time.Time `json:"last_failure_time,omitempty"`
	AwaitingAck     bool      `json:"awaiting_ack"`
}

// ReminderView is what the dashboard consumes.
type ReminderView struct {
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Message     string          `json:"message"`
	NextTrigger time.Time       `json:"next_trigger"`
	Profile     ReminderProfile `json:"profile"`
	Mode        ReminderMode    `json:"mode"`
	VisibleTo   []string        `json:"visible_to,omitempty"`
	RepeatCount int             `json:"repeat_count"`
	Completed   bool            `json:"completed"`
	Cancelled   bool            `json:"cancelled"`
	Speaker     string          `json:"speaker_entity,omitempty"`
	Phone       string          `json:"phone_notifier,omitempty"`
	AwaitingAck bool            `json:"awaiting_ack"`
}

func (q QuietHoursConfig) isQuiet(now time.Time) bool {
	if !q.Enabled {
		return false
	}

	startMinute, err := parseClockMinutes(q.Start)
	if err != nil {
		return false
	}

	endMinute, err := parseClockMinutes(q.End)
	if err != nil {
		return false
	}

	currentMinute := now.Hour()*60 + now.Minute()

	if startMinute <= endMinute {
		return currentMinute >= startMinute && currentMinute < endMinute
	}

	// Wraps around midnight
	return currentMinute >= startMinute || currentMinute < endMinute
}

func (q QuietHoursConfig) nextWindowEnd(now time.Time) time.Time {
	endMinute, err := parseClockMinutes(q.End)
	if err != nil {
		return now.Add(time.Hour)
	}

	dayMinutes := now.Hour()*60 + now.Minute()
	deltaMinutes := endMinute - dayMinutes
	if deltaMinutes <= 0 {
		deltaMinutes += 24 * 60
	}

	return now.Add(time.Duration(deltaMinutes) * time.Minute)
}

func parseClockMinutes(value string) (int, error) {
	if value == "" {
		return 0, fmt.Errorf("empty clock string")
	}

	parts := strings.Split(value, ":")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid clock string: %s", value)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid hour: %w", err)
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid minute: %w", err)
	}

	return hour*60 + minute, nil
}

func (cfg ReminderDefinition) nextInterval() time.Duration {
	prev := cfg.InitialRepeatMin
	return time.Duration(prev) * time.Minute
}
