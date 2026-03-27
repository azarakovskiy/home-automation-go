package laptop

import (
	"time"

	"home-go/internal/domain/charging"
	"home-go/internal/tech/homeassistant/entities"
)

// Profile optimized for workday uptime while avoiding expensive hours.
var Profile = charging.ChargingProfile{
	Name:               "Laptop",
	Strategy:           charging.StrategyCriticalUptime,
	TotalDuration:      1 * time.Hour,
	WindowSize:         12 * time.Hour,
	CriticalHoursStart: 9,
	CriticalHoursEnd:   19,
	DrainRate:          3 * time.Hour,
	BatteryEntity:      entities.CustomSensors.OfficeLaptopWorkInternalBatteryLevel,
	MinBatteryPercent:  10,
	Description:        "Work laptop - charged by 9 AM, drains through work, charges overnight",
}
