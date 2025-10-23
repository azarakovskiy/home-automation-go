package dishwasher

import (
	"fmt"
	"time"

	"home-go/scheduler/optimizer"
)

// GetProfileForMode returns the optimizer profile for a specific dishwasher mode
// Device modes are defined here, but the Profile struct is generic from the optimizer package
// Note: MinSavingsPercent is now calculated dynamically by the optimizer based on MaxDelayHours
func GetProfileForMode(mode Mode) (optimizer.Profile, error) {
	profiles := map[Mode]optimizer.Profile{
		ModeEco: {
			Mode:     string(ModeEco),
			Duration: 4 * time.Hour,
			// Pre-wash (low), Main wash (max - most important), Dry (medium)
			StageWeights: []float64{optimizer.WeightLow, optimizer.WeightMax, optimizer.WeightMedium},
			PowerKW:      0.8,
		},
		ModeAuto: {
			Mode:     string(ModeAuto),
			Duration: 137 * time.Minute, // Measured: exactly 137 minutes
			// Stage weights based on actual measured power consumption pattern (7 stages)
			StageWeights: []float64{
				optimizer.WeightMax,  // 1. Initial heating (weight: 1 normalized to 5)
				optimizer.WeightMax,  // 2. Main wash high power (weight: 5)
				optimizer.WeightMax,  // 3. Main wash continued (weight: 5)
				optimizer.WeightLow,  // 4. Rinse phase (weight: 2)
				optimizer.WeightHigh, // 5. Heating for drying (weight: 4)
				optimizer.WeightHigh, // 6. Drying continued (weight: 4)
				optimizer.WeightLow,  // 7. Final drying (weight: 2)
			},
			PowerKW: 2.0, // Measured: similar to AutoQuick
		},
		ModeAutoQuick: {
			Mode:     string(ModeAutoQuick),
			Duration: 70 * time.Minute, // Measured: ~70 minutes with VarioDry quick option
			// Stage weights based on actual power consumption pattern (13 stages)
			// Using standard weight levels to approximate the measured power curve
			StageWeights: []float64{
				optimizer.WeightMedium, // 1. First heating/wash
				optimizer.WeightHigh,   // 2. Continued heating
				optimizer.WeightHigh,   // 3. Main wash (peak power)
				optimizer.WeightIdle,   // 4. Pause/drain
				optimizer.WeightIdle,   // 5. Pause/drain
				optimizer.WeightHigh,   // 6. Major stage
				optimizer.WeightLow,    // 7. Medium stage
				optimizer.WeightLow,    // 8. Medium stage
				optimizer.WeightLow,    // 9. Medium stage
				optimizer.WeightMax,    // 10. Major heating stage (peak)
				optimizer.WeightHigh,   // 11. Drying stage
				optimizer.WeightLow,    // 12. Final medium stage
				optimizer.WeightIdle,   // 13. Final small stage
			},
			PowerKW: 2.0, // Measured: ~2000W during active washing
		},
		ModeIntensive: {
			Mode:     string(ModeIntensive),
			Duration: 3 * time.Hour,
			// All stages use high power
			StageWeights: []float64{optimizer.WeightMedium, optimizer.WeightMax, optimizer.WeightMedium},
			PowerKW:      1.5,
		},
		ModeQuick: {
			Mode:     string(ModeQuick),
			Duration: 1 * time.Hour,
			// Pre-wash (medium), Main wash (max - critical), No dry
			StageWeights: []float64{optimizer.WeightMedium, optimizer.WeightMax, optimizer.WeightIdle},
			PowerKW:      2.0,
		},
	}

	profile, ok := profiles[mode]
	if !ok {
		return optimizer.Profile{}, fmt.Errorf("unknown dishwasher mode: %s", mode)
	}

	return profile, nil
}
