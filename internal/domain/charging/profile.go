package charging

import (
	"time"

	"home-go/optimization/optimizer"
)

// ChargingStrategy defines how the device should be charged.
type ChargingStrategy string

const (
	// StrategyOpportunistic charges only during cheapest slots.
	StrategyOpportunistic ChargingStrategy = "opportunistic"

	// StrategyCriticalUptime ensures device charge during critical hours.
	StrategyCriticalUptime ChargingStrategy = "critical_uptime"
)

// ChargingProfile defines generic charging behavior for different device types.
type ChargingProfile struct {
	Name          string
	Strategy      ChargingStrategy
	TotalDuration time.Duration
	WindowSize    time.Duration

	CriticalHoursStart int
	CriticalHoursEnd   int
	DrainRate          time.Duration
	BatteryEntity      string
	MinBatteryPercent  int

	Description string
}

// ToOptimizerRequest converts profile to optimizer request.
func (p ChargingProfile) ToOptimizerRequest() optimizer.CheapestHoursRequest {
	minBattery := p.MinBatteryPercent
	if minBattery == 0 {
		minBattery = 20
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
	}
}
