package charger

import (
	"time"

	"home-go/scheduler/optimizer"
)

// ChargingProfile defines charging behavior for different device types
type ChargingProfile struct {
	Name          string
	TotalDuration time.Duration
	WindowSize    time.Duration
	Description   string
}

// ToOptimizerRequest converts profile to optimizer request
func (p ChargingProfile) ToOptimizerRequest() optimizer.CheapestHoursRequest {
	return optimizer.CheapestHoursRequest{
		TotalDuration: p.TotalDuration,
		WindowSize:    p.WindowSize,
	}
}

// Predefined charging profiles for active devices
var (
	// LaptopProfile - typical laptop charging: 6h needed in 12h window
	LaptopProfile = ChargingProfile{
		Name:          "Laptop",
		TotalDuration: 6 * time.Hour,
		WindowSize:    12 * time.Hour,
		Description:   "Extended charging for laptops",
	}

	// VacuumProfile - quick charging: 1h needed in 12h window
	VacuumProfile = ChargingProfile{
		Name:          "Vacuum",
		TotalDuration: 1 * time.Hour,
		WindowSize:    12 * time.Hour,
		Description:   "Quick charging for vacuum cleaners",
	}
)
