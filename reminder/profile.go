package reminder

import (
	"math"
	"time"
)

// nextIntervalDuration returns the delay before the next reminder fires.
// The boolean indicates whether the reminder should be cancelled after the current send.
func nextIntervalDuration(profile ReminderProfile, cfg ReminderDefinition, previousMinutes int) (time.Duration, bool) {
	if previousMinutes <= 0 {
		previousMinutes = cfg.InitialRepeatMin
	}

	switch profile {
	case ProfileAnnoying:
		next := int(math.Ceil(float64(previousMinutes) * annoyingDecayFactor))
		if next < cfg.MinRepeatMin {
			next = cfg.MinRepeatMin
		}
		return time.Duration(next) * time.Minute, false
	case ProfileQuiet:
		next := int(math.Ceil(float64(previousMinutes) * quietGrowthFactor))
		if next >= cfg.MaxRepeatMin {
			return 0, true
		}
		return time.Duration(next) * time.Minute, false
	default:
		return time.Duration(cfg.InitialRepeatMin) * time.Minute, false
	}
}
