package dishwasher

// Mode represents dishwasher operating modes.
type Mode string

const (
	ModeAuto      Mode = "auto"
	ModeAutoQuick Mode = "auto_quick"
	ModeCancel    Mode = "cancel"
)
