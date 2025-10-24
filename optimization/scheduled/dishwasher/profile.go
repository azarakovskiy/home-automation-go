package dishwasher

import (
	"fmt"
	"time"

	"home-go/optimization/scheduled"
)

// GetProfileForMode returns the optimizer profile for a specific dishwasher mode
// Device modes are defined here, but the Profile struct is generic from the scheduler package
// Note: MinSavingsPercent is now calculated dynamically by the optimizer based on MaxDelayHours
func GetProfileForMode(mode Mode) (scheduled.Profile, error) {
	profiles := map[Mode]scheduled.Profile{
		ModeEco: {
			Mode:     string(ModeEco),
			Duration: 4 * time.Hour,
			// Pre-wash (low), Main wash (max - most important), Dry (medium)
			StageWeights: []float64{scheduled.WeightLow, scheduled.WeightMax, scheduled.WeightMedium},
			PowerKW:      0.8,
		},
		ModeAuto: {
			Mode:     string(ModeAuto),
			Duration: 137 * time.Minute, // Measured: exactly 137 minutes
			// Stage weights based on actual measured power consumption pattern (7 stages)
			StageWeights: []float64{
				scheduled.WeightMax,  // 1. Initial heating (weight: 1 normalized to 5)
				scheduled.WeightMax,  // 2. Main wash high power (weight: 5)
				scheduled.WeightMax,  // 3. Main wash continued (weight: 5)
				scheduled.WeightLow,  // 4. Rinse phase (weight: 2)
				scheduled.WeightHigh, // 5. Heating for drying (weight: 4)
				scheduled.WeightHigh, // 6. Drying continued (weight: 4)
				scheduled.WeightLow,  // 7. Final drying (weight: 2)
			},
			PowerKW: 2.0, // Measured: similar to AutoQuick
		},
		ModeAutoQuick: {
			Mode:     string(ModeAutoQuick),
			Duration: 70 * time.Minute, // Measured: ~70 minutes with VarioDry quick option
			// Stage weights based on actual power consumption pattern (13 stages)
			// Using standard weight levels to approximate the measured power curve
			StageWeights: []float64{
				scheduled.WeightMedium, // 1. First heating/wash
				scheduled.WeightHigh,   // 2. Continued heating
				scheduled.WeightHigh,   // 3. Main wash (peak power)
				scheduled.WeightIdle,   // 4. Pause/drain
				scheduled.WeightIdle,   // 5. Pause/drain
				scheduled.WeightHigh,   // 6. Major stage
				scheduled.WeightLow,    // 7. Medium stage
				scheduled.WeightLow,    // 8. Medium stage
				scheduled.WeightLow,    // 9. Medium stage
				scheduled.WeightMax,    // 10. Major heating stage (peak)
				scheduled.WeightHigh,   // 11. Drying stage
				scheduled.WeightLow,    // 12. Final medium stage
				scheduled.WeightIdle,   // 13. Final small stage
			},
			PowerKW: 2.0, // Measured: ~2000W during active washing
		},
		ModeIntensive: {
			Mode:     string(ModeIntensive),
			Duration: 3 * time.Hour,
			// All stages use high power
			StageWeights: []float64{scheduled.WeightMedium, scheduled.WeightMax, scheduled.WeightMedium},
			PowerKW:      1.5,
		},
		ModeQuick: {
			Mode:     string(ModeQuick),
			Duration: 1 * time.Hour,
			// Pre-wash (medium), Main wash (max - critical), No dry
			StageWeights: []float64{scheduled.WeightMedium, scheduled.WeightMax, scheduled.WeightIdle},
			PowerKW:      2.0,
		},
	}

	profile, ok := profiles[mode]
	if !ok {
		return scheduled.Profile{}, fmt.Errorf("unknown dishwasher mode: %s", mode)
	}

	return profile, nil
}
