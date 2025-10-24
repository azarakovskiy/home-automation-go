package dishwasher

// Mode represents dishwasher operating modes
type Mode string

// Dishwasher-specific modes
const (
	ModeEco       Mode = "eco"
	ModeAuto      Mode = "auto"
	ModeAutoQuick Mode = "auto_quick" // Auto with VarioDry quick option
	ModeIntensive Mode = "intensive"
	ModeQuick     Mode = "quick"
)
