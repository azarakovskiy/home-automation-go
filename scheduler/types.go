package scheduler

// ScheduleRequest is the generic event data for scheduling any device
type ScheduleRequest struct {
	Device        string `json:"device"`
	Mode          Mode   `json:"mode"`
	MaxDelayHours int    `json:"max_delay_hours"`
}

// Mode is a generic type for device operating modes
type Mode string
