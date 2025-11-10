package dishwasher

// Mode represents dishwasher operating modes
type Mode string

// Dishwasher-specific modes
const (
	ModeAuto      Mode = "auto"
	ModeAutoQuick Mode = "auto_quick" // Auto with VarioDry quick option
	ModeCancel    Mode = "cancel"     // Virtual mode used to cancel pending schedules
)
