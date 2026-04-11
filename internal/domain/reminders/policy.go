package reminders

import "time"

// EscalationPolicy defines how aggressively a reminder repeats after firing.
type EscalationPolicy struct {
	InitialDelay   time.Duration
	RepeatInterval time.Duration
	MaxRepeats     int // 0 = unlimited
}

var (
	EscalationQuiet    = EscalationPolicy{InitialDelay: 15 * time.Minute, RepeatInterval: 30 * time.Minute, MaxRepeats: 3}
	EscalationNormal   = EscalationPolicy{InitialDelay: 5 * time.Minute, RepeatInterval: 10 * time.Minute, MaxRepeats: 0}
	EscalationAnnoying = EscalationPolicy{InitialDelay: 1 * time.Minute, RepeatInterval: 2 * time.Minute, MaxRepeats: 0}
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
