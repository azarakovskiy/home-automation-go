package scheduled

// ScheduleRequest is the generic event data for scheduling any device
type ScheduleRequest struct {
	Device        string `json:"device"`
	Mode          string `json:"mode"`
	MaxDelayHours int    `json:"max_delay_hours"`
}
