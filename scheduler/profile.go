package scheduler

import (
	"time"

	"home-go/scheduler/optimizer"
)

// Standard stage weight levels for consistent profiling across devices
// These represent the importance of optimizing each stage (higher = more important)
const (
	WeightIdle   = 0.0  // Idle/pause stages with minimal power consumption
	WeightLow    = 0.25 // Low power stages (draining, pre-rinse)
	WeightMedium = 0.50 // Medium power stages (washing, rinsing)
	WeightHigh   = 0.75 // High power stages (heating water, intensive wash)
	WeightMax    = 1.0  // Maximum power stages (main heating, intensive drying)
)

// Profile is a generic device profile implementation
// Devices can use this directly or create their own implementation of DeviceProfile
type Profile struct {
	Mode         string
	Duration     time.Duration
	StageWeights []float64
	PowerKW      float64
}

// Verify Profile implements the interface at compile time
var _ optimizer.DeviceProfile = (*Profile)(nil)

func (p Profile) GetDuration() time.Duration {
	return p.Duration
}

func (p Profile) GetStageWeights() []float64 {
	return p.StageWeights
}

func (p Profile) GetPowerKW() float64 {
	return p.PowerKW
}

func (p Profile) GetMode() string {
	return p.Mode
}
