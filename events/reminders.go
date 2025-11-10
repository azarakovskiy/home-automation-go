package events

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// QuietHoursPayload mirrors the dashboard payload for quiet hours configuration.
type QuietHoursPayload struct {
	Enabled bool   `json:"enabled"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

// OptionalInt unmarshals integers that might arrive as numbers or quoted strings.
type OptionalInt struct {
	value *int
}

// Ptr returns the pointer to the parsed integer, if any.
func (o OptionalInt) Ptr() *int {
	return o.value
}

// UnmarshalJSON accepts numeric literals, quoted strings, or null.
func (o *OptionalInt) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		o.value = nil
		return nil
	}

	var number int
	if data[0] == '"' {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			o.value = nil
			return nil
		}
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return err
		}
		number = parsed
	} else {
		if err := json.Unmarshal(data, &number); err != nil {
			return err
		}
	}

	o.value = &number
	return nil
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
	InitialRepeatMin    OptionalInt        `json:"initial_repeat_minutes,omitempty"`
	MinRepeatMin        OptionalInt        `json:"min_repeat_minutes,omitempty"`
	MaxRepeatMin        OptionalInt        `json:"max_repeat_minutes,omitempty"`
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
