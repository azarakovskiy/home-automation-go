package dishwasher

import (
	"fmt"
	"time"
)

const (
	weightIdle   = 0.0
	weightLow    = 0.25
	weightMedium = 0.50
	weightHigh   = 0.75
	weightMax    = 1.0
)

// Profile is the dishwasher-specific optimizer profile.
type Profile struct {
	Mode         string
	Duration     time.Duration
	StageWeights []float64
	PowerKW      float64
}

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

// GetProfileForMode returns the optimizer profile for a specific dishwasher mode.
func GetProfileForMode(mode Mode) (Profile, error) {
	profiles := map[Mode]Profile{
		ModeAuto: {
			Mode:     string(ModeAuto),
			Duration: 137 * time.Minute,
			StageWeights: []float64{
				weightMax,
				weightMax,
				weightMax,
				weightLow,
				weightHigh,
				weightHigh,
				weightLow,
			},
			PowerKW: 2.0,
		},
		ModeAutoQuick: {
			Mode:     string(ModeAutoQuick),
			Duration: 70 * time.Minute,
			StageWeights: []float64{
				weightMedium,
				weightHigh,
				weightHigh,
				weightIdle,
				weightIdle,
				weightHigh,
				weightLow,
				weightLow,
				weightLow,
				weightMax,
				weightHigh,
				weightLow,
				weightIdle,
			},
			PowerKW: 2.0,
		},
	}

	profile, ok := profiles[mode]
	if !ok {
		return Profile{}, fmt.Errorf("unknown dishwasher mode: %s", mode)
	}

	return profile, nil
}
