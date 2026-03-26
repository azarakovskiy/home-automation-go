package continuous

import (
	"time"

	"home-go/entities"
	domaincharging "home-go/internal/domain/charging"
)

type ChargingStrategy = domaincharging.ChargingStrategy
type ChargingProfile = domaincharging.ChargingProfile

const (
	StrategyOpportunistic  = domaincharging.StrategyOpportunistic
	StrategyCriticalUptime = domaincharging.StrategyCriticalUptime
)

// Predefined charging profiles for active devices
var (
	// LaptopProfile - critical uptime during work hours with adaptive battery management
	// Optimized for typical work pattern:
	// - Fully charged by 9 AM (before morning price peak)
	// - Drain through morning peak (9 AM) and work hours
	// - Light usage during lunch (12-13), opportunistic charging if cheap
	// - Continue working through afternoon
	// - Drain through evening peak (18:00)
	// - Can reach minimum (10%) by 19:00, then charge overnight at cheap rates
	// - Avoids charging during expensive peak hours (9:00, 18:00)
	// Uses actual battery sensor from Home Assistant companion app.
	LaptopProfile = ChargingProfile{
		Name:               "Laptop",
		Strategy:           StrategyCriticalUptime,
		TotalDuration:      1 * time.Hour,  // Measured: 20% → 100% in ~1h
		WindowSize:         12 * time.Hour, // Find cheapest 1h within 12h window (night + lunch)
		CriticalHoursStart: 9,              // Work starts at 9 AM (morning peak)
		CriticalHoursEnd:   19,             // Work ends at 19:00 (after evening peak)
		DrainRate:          3 * time.Hour,  // Measured: 100% → 20% in 3h
		BatteryEntity:      entities.CustomSensors.OfficeLaptopWorkInternalBatteryLevel,
		MinBatteryPercent:  10, // Allow aggressive drain to 10% by end of work day
		Description:        "Work laptop - charged by 9 AM, drains through work, charges overnight",
	}

	// VacuumProfile - opportunistic charging, no critical uptime
	// Just charges during cheapest available slots
	VacuumProfile = ChargingProfile{
		Name:          "Vacuum",
		Strategy:      StrategyOpportunistic,
		TotalDuration: 1 * time.Hour,
		WindowSize:    12 * time.Hour,
		Description:   "Opportunistic charging during cheap periods",
	}
)
