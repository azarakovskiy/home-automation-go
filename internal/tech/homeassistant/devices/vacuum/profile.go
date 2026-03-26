package vacuum

import (
	"time"

	"home-go/internal/domain/charging"
)

// Profile optimized for opportunistic charging during cheap periods.
var Profile = charging.ChargingProfile{
	Name:          "Vacuum",
	Strategy:      charging.StrategyOpportunistic,
	TotalDuration: 1 * time.Hour,
	WindowSize:    12 * time.Hour,
	Description:   "Opportunistic charging during cheap periods",
}
