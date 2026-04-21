package dishwasher

// ScheduleRequest is the dishwasher-specific event payload for delayed start requests.
type ScheduleRequest struct {
	Device        string `json:"device"`
	Mode          string `json:"mode"`
	MaxDelayHours int    `json:"max_delay_hours"`
}
