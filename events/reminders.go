package events

// QuietHoursPayload mirrors the dashboard payload for quiet hours configuration.
type QuietHoursPayload struct {
	Enabled bool   `json:"enabled"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

// ReminderCreateEvent describes the payload coming from the dashboard when a reminder is created or updated.
type ReminderCreateEvent struct {
	ID                  string             `json:"id"`
	Title               string             `json:"title"`
	Message             string             `json:"message"`
	Profile             string             `json:"profile"`
	Mode                string             `json:"mode"`
	OneTime             *bool              `json:"one_time,omitempty"`
	StartTime           string             `json:"start_time"`
	InitialRepeatMin    *int               `json:"initial_repeat_minutes,omitempty"`
	MinRepeatMin        *int               `json:"min_repeat_minutes,omitempty"`
	MaxRepeatMin        *int               `json:"max_repeat_minutes,omitempty"`
	SpeakerEntity       string             `json:"speaker_entity,omitempty"`
	PhoneNotifier       string             `json:"phone_notifier,omitempty"`
	VisibleTo           []string           `json:"visible_to,omitempty"`
	NightModeAllowed    *bool              `json:"night_mode_allowed,omitempty"`
	PresenceRequired    *bool              `json:"presence_required,omitempty"`
	QuietHours          *QuietHoursPayload `json:"quiet_hours,omitempty"`
	Metadata            map[string]string  `json:"metadata,omitempty"`
	InitialDelayMinutes *int               `json:"initial_delay_minutes,omitempty"`
}

// ReminderAckEvent acknowledges a pending reminder.
type ReminderAckEvent struct {
	ID   string `json:"id"`
	User string `json:"user,omitempty"`
}

// ReminderDeleteEvent removes a reminder configuration entirely.
type ReminderDeleteEvent struct {
	ID string `json:"id"`
}
