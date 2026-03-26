package notifications

import (
	"fmt"
	"time"
)

// Event describes a user-facing notification independent of transport.
type Event struct {
	Device  string                 `json:"device"`
	Type    string                 `json:"type"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// FormatTimeForSpeech converts a time to a natural spoken format.
func FormatTimeForSpeech(t time.Time) string {
	hour := t.Hour()
	minute := t.Minute()

	if hour == 0 && minute == 0 {
		return "midnight"
	}
	if hour == 12 && minute == 0 {
		return "noon"
	}

	period := "AM"
	displayHour := hour
	if hour >= 12 {
		period = "PM"
		if hour > 12 {
			displayHour = hour - 12
		}
	}
	if displayHour == 0 {
		displayHour = 12
	}

	if minute == 0 {
		return fmt.Sprintf("%d %s", displayHour, period)
	}
	return fmt.Sprintf("%d:%02d %s", displayHour, minute, period)
}
