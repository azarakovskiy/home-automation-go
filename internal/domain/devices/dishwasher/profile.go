package dishwasher

import (
	"fmt"
	"time"

	"home-go/internal/domain/scheduler"
)

// GetProfileForMode returns the optimizer profile for a specific dishwasher mode.
func GetProfileForMode(mode Mode) (scheduler.Profile, error) {
	profiles := map[Mode]scheduler.Profile{
		ModeAuto: {
			Mode:     string(ModeAuto),
			Duration: 137 * time.Minute,
			StageWeights: []float64{
				scheduler.WeightMax,
				scheduler.WeightMax,
				scheduler.WeightMax,
				scheduler.WeightLow,
				scheduler.WeightHigh,
				scheduler.WeightHigh,
				scheduler.WeightLow,
			},
			PowerKW: 2.0,
		},
		ModeAutoQuick: {
			Mode:     string(ModeAutoQuick),
			Duration: 70 * time.Minute,
			StageWeights: []float64{
				scheduler.WeightMedium,
				scheduler.WeightHigh,
				scheduler.WeightHigh,
				scheduler.WeightIdle,
				scheduler.WeightIdle,
				scheduler.WeightHigh,
				scheduler.WeightLow,
				scheduler.WeightLow,
				scheduler.WeightLow,
				scheduler.WeightMax,
				scheduler.WeightHigh,
				scheduler.WeightLow,
				scheduler.WeightIdle,
			},
			PowerKW: 2.0,
		},
	}

	profile, ok := profiles[mode]
	if !ok {
		return scheduler.Profile{}, fmt.Errorf("unknown dishwasher mode: %s", mode)
	}

	return profile, nil
}
