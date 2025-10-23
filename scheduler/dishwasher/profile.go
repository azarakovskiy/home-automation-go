package dishwasher

import (
	"fmt"
	"time"

	"home-go/scheduler"
)

// GetProfileForMode returns the optimizer profile for a specific dishwasher mode
// Device modes are defined here, but the Profile struct is generic from the scheduler package
// Note: MinSavingsPercent is now calculated dynamically by the optimizer based on MaxDelayHours
func GetProfileForMode(mode Mode) (scheduler.Profile, error) {
	profiles := map[Mode]scheduler.Profile{
		ModeEco: {
			Mode:     string(ModeEco),
			Duration: 4 * time.Hour,
			// Pre-wash (low), Main wash (max - most important), Dry (medium)
			StageWeights: []float64{scheduler.WeightLow, scheduler.WeightMax, scheduler.WeightMedium},
			PowerKW:      0.8,
		},
		ModeAuto: {
			Mode:     string(ModeAuto),
			Duration: 137 * time.Minute, // Measured: exactly 137 minutes
			// Stage weights based on actual measured power consumption pattern (7 stages)
			StageWeights: []float64{
				scheduler.WeightMax,  // 1. Initial heating (weight: 1 normalized to 5)
				scheduler.WeightMax,  // 2. Main wash high power (weight: 5)
				scheduler.WeightMax,  // 3. Main wash continued (weight: 5)
				scheduler.WeightLow,  // 4. Rinse phase (weight: 2)
				scheduler.WeightHigh, // 5. Heating for drying (weight: 4)
				scheduler.WeightHigh, // 6. Drying continued (weight: 4)
				scheduler.WeightLow,  // 7. Final drying (weight: 2)
			},
			PowerKW: 2.0, // Measured: similar to AutoQuick
		},
		ModeAutoQuick: {
			Mode:     string(ModeAutoQuick),
			Duration: 70 * time.Minute, // Measured: ~70 minutes with VarioDry quick option
			// Stage weights based on actual power consumption pattern (13 stages)
			// Using standard weight levels to approximate the measured power curve
			StageWeights: []float64{
				scheduler.WeightMedium, // 1. First heating/wash
				scheduler.WeightHigh,   // 2. Continued heating
				scheduler.WeightHigh,   // 3. Main wash (peak power)
				scheduler.WeightIdle,   // 4. Pause/drain
				scheduler.WeightIdle,   // 5. Pause/drain
				scheduler.WeightHigh,   // 6. Major stage
				scheduler.WeightLow,    // 7. Medium stage
				scheduler.WeightLow,    // 8. Medium stage
				scheduler.WeightLow,    // 9. Medium stage
				scheduler.WeightMax,    // 10. Major heating stage (peak)
				scheduler.WeightHigh,   // 11. Drying stage
				scheduler.WeightLow,    // 12. Final medium stage
				scheduler.WeightIdle,   // 13. Final small stage
			},
			PowerKW: 2.0, // Measured: ~2000W during active washing
		},
		ModeIntensive: {
			Mode:     string(ModeIntensive),
			Duration: 3 * time.Hour,
			// All stages use high power
			StageWeights: []float64{scheduler.WeightMedium, scheduler.WeightMax, scheduler.WeightMedium},
			PowerKW:      1.5,
		},
		ModeQuick: {
			Mode:     string(ModeQuick),
			Duration: 1 * time.Hour,
			// Pre-wash (medium), Main wash (max - critical), No dry
			StageWeights: []float64{scheduler.WeightMedium, scheduler.WeightMax, scheduler.WeightIdle},
			PowerKW:      2.0,
		},
	}

	profile, ok := profiles[mode]
	if !ok {
		return scheduler.Profile{}, fmt.Errorf("unknown dishwasher mode: %s", mode)
	}

	return profile, nil
}
