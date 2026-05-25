package reminders

// CreateCommand is the MQTT payload to create a new reminder.
type CreateCommand struct {
	ID          string   `json:"id"`
	Targets     []string `json:"targets"`
	Message     string   `json:"message"`
	Owner       string   `json:"owner"`
	Source      string   `json:"source"`
	TriggerAt   string   `json:"trigger_at"`            // RFC3339
	RecurEvery  string   `json:"recur_every,omitempty"` // Go duration string, e.g. "1h"
	ValidUntil  string   `json:"valid_until,omitempty"` // RFC3339
	RequiresAck bool     `json:"requires_ack"`
	Profile     string   `json:"profile"` // "quiet" | "normal" | "annoying"
}

// AckCommand is the MQTT payload to acknowledge a reminder.
type AckCommand struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
}

// DeleteCommand is the MQTT payload to delete a reminder.
type DeleteCommand struct {
	ID string `json:"id"`
}
