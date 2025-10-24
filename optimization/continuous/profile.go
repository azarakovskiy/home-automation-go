package continuous

import (
	"time"

	"home-go/entities"
	"home-go/optimization/optimizer"
)

// ChargingStrategy defines how the device should be charged
type ChargingStrategy string

const (
	// StrategyOpportunistic charges only during cheapest slots (vacuum, non-critical devices)
	StrategyOpportunistic ChargingStrategy = "opportunistic"

	// StrategyCriticalUptime ensures device is charged during critical hours (laptop, work devices)
	// Pre-charges before critical periods if cheaper, allows drain during expensive critical hours
	StrategyCriticalUptime ChargingStrategy = "critical_uptime"
)

// ChargingProfile defines charging behavior for different device types
type ChargingProfile struct {
	Name          string
	Strategy      ChargingStrategy
	TotalDuration time.Duration // Total charging duration needed per cycle
	WindowSize    time.Duration // Time window to optimize within

	// For StrategyCriticalUptime only:
	CriticalHoursStart int           // Hour when device must be available (e.g., 9 for 9 AM)
	CriticalHoursEnd   int           // Hour when critical period ends (e.g., 17 for 5 PM)
	DrainRate          time.Duration // How long device can run on full charge (e.g., 2h)
	BatteryEntity      string        // Optional: HA entity for battery level (e.g., "sensor.laptop_battery_level")
	MinBatteryPercent  int           // Charge during critical hours if battery drops below this (default: 20)

	Description string
}

// ToOptimizerRequest converts profile to optimizer request
func (p ChargingProfile) ToOptimizerRequest() optimizer.CheapestHoursRequest {
	minBattery := p.MinBatteryPercent
	if minBattery == 0 {
		minBattery = 20 // Default to 20% minimum
	}

	return optimizer.CheapestHoursRequest{
		DeviceName:         p.Name,
		TotalDuration:      p.TotalDuration,
		WindowSize:         p.WindowSize,
		CriticalHoursStart: p.CriticalHoursStart,
		CriticalHoursEnd:   p.CriticalHoursEnd,
		DrainRate:          p.DrainRate,
		BatteryEntity:      p.BatteryEntity,
		MinBatteryPercent:  minBattery,
		// Strategy is auto-detected by optimizer based on CriticalHoursStart/End
	}
}

// Predefined charging profiles for active devices
var (
	// LaptopProfile - critical uptime during work hours (10-18)
	// Must be charged before work starts. Can charge opportunistically outside work hours.
	// Working drains battery (~2h usage), needs pre-charging to ensure availability.
	// Uses actual battery sensor from Home Assistant companion app.
	LaptopProfile = ChargingProfile{
		Name:               "Laptop",
		Strategy:           StrategyCriticalUptime,
		TotalDuration:      6 * time.Hour,  // Needs 6h total charging per cycle
		WindowSize:         12 * time.Hour, // Find cheap slots in 12h window (typically night)
		CriticalHoursStart: 10,             // Must be available 10 AM - 6 PM
		CriticalHoursEnd:   18,
		DrainRate:          2 * time.Hour,                                        // 2 hours of work drains battery
		BatteryEntity:      entities.Sensor.OfficeLaptopWorkInternalBatteryLevel, // HA companion app sensor (auto-generated)
		MinBatteryPercent:  20,                                                   // Charge during work if battery < 20%
		Description:        "Work laptop - ensure charged before 10 AM",
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
