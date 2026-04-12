package reminders

// CreateReminderEvent is the payload fired by a Home Assistant automation
// to request a new reminder.
type CreateReminderEvent struct {
	// ID is the caller-assigned stable reminder identifier.
	ID string `json:"id"`
	// Targets is the list of user IDs that should receive the reminder.
	Targets []string `json:"targets"`
	// Message is the human-readable reminder text.
	Message string `json:"message"`
	// Owner is the user who created the reminder.
	Owner string `json:"owner"`
	// Source identifies the system or automation that created the reminder.
	Source string `json:"source"`
	// TriggerAt is the first fire time in RFC3339 format.
	TriggerAt string `json:"trigger_at"`
	// RecurEvery is an optional Go duration string (e.g. "1h", "30m") for recurring reminders.
	RecurEvery string `json:"recur_every,omitempty"`
	// ValidUntil is an optional expiry time in RFC3339 format.
	ValidUntil string `json:"valid_until,omitempty"`
	// RequiresAck controls whether users must explicitly acknowledge the reminder.
	RequiresAck bool `json:"requires_ack"`
	// CompletionPolicy is "all_targets_ack" or "any_target_ack".
	CompletionPolicy string `json:"completion_policy"`
	// Profile is "quiet", "normal", or "annoying".
	Profile string `json:"profile"`
}

// AckReminderEvent is the payload fired to acknowledge a reminder for a specific user.
type AckReminderEvent struct {
	// ID is the reminder to acknowledge.
	ID string `json:"id"`
	// UserID is the user who is acknowledging.
	UserID string `json:"user_id"`
}

// DeleteReminderEvent is the payload fired to delete a reminder.
type DeleteReminderEvent struct {
	// ID is the reminder to delete.
	ID string `json:"id"`
}
