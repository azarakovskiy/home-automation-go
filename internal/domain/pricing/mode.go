package pricing

// ModeProvider reports household operating mode.
// Currently sourced from HA entities; designed for future Go-owned state.
type ModeProvider interface {
	IsNight() (bool, error)
	IsAway() (bool, error)
}
