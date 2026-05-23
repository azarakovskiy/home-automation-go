package reminders

import "time"

// EscalationPolicy defines how aggressively a reminder repeats after firing.
type EscalationPolicy struct {
	InitialDelay   time.Duration
	RepeatInterval time.Duration
	DecreaseStep   time.Duration // > 0 for decreasing intervals (annoying profile)
	MinInterval    time.Duration // lower bound when DecreaseStep > 0
	MaxRepeats     int           // 0 = unlimited
}

var (
	EscalationQuiet = EscalationPolicy{
		InitialDelay:   30 * time.Minute,
		RepeatInterval: 1 * time.Hour,
		MaxRepeats:     3,
	}
	EscalationNormal = EscalationPolicy{
		InitialDelay:   15 * time.Minute,
		RepeatInterval: 15 * time.Minute,
		MaxRepeats:     0,
	}
	EscalationAnnoying = EscalationPolicy{
		InitialDelay:   15 * time.Minute,
		RepeatInterval: 15 * time.Minute,
		DecreaseStep:   2 * time.Minute,
		MinInterval:    5 * time.Minute,
		MaxRepeats:     0,
	}
)

// PolicyForProfile returns the escalation preset for the given profile.
func PolicyForProfile(p Profile) EscalationPolicy {
	switch p {
	case ProfileQuiet:
		return EscalationQuiet
	case ProfileAnnoying:
		return EscalationAnnoying
	default:
		return EscalationNormal
	}
}
