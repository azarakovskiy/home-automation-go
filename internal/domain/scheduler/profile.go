package scheduler

import (
	"time"

	"home-go/optimization/optimizer"
)

const (
	WeightIdle   = 0.0
	WeightLow    = 0.25
	WeightMedium = 0.50
	WeightHigh   = 0.75
	WeightMax    = 1.0
)

// Profile is a generic device profile implementation.
type Profile struct {
	Mode         string
	Duration     time.Duration
	StageWeights []float64
	PowerKW      float64
}

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
