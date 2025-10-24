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
	CriticalHoursEnd   int           // Hour when critical period ends (e.g., 19 for 7 PM)
	DrainRate          time.Duration // How long device can run on full charge (e.g., 8h for full workday)
	BatteryEntity      string        // Optional: HA entity for battery level (e.g., "sensor.laptop_battery_level")
	MinBatteryPercent  int           // Minimum battery during critical hours (e.g., 10% - allows aggressive drain)

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
		TotalDuration:      6 * time.Hour,  // Needs 6h total charging per cycle
		WindowSize:         12 * time.Hour, // Find cheap slots in 12h window (typically night + lunch)
		CriticalHoursStart: 9,              // Work starts at 9 AM (morning peak)
		CriticalHoursEnd:   19,             // Work ends at 19:00 (after evening peak)
		DrainRate:          8 * time.Hour,  // Can run ~8 hours on full charge (full workday)
		BatteryEntity:      entities.Sensor.OfficeLaptopWorkInternalBatteryLevel,
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
