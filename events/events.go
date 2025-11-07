package events

type ScheduledStart struct {
	Device        Device `json:"device"`
	Mode          Mode   `json:"mode"`
	MaxDelayHours int    `json:"max_delay_hours"`
}

type Device string

var DeviceDishwasher = "dishwasher"

type Mode string

var ModeDishwasherAuto = "auto"
