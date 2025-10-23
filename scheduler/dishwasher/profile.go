package dishwasher

import (
	"fmt"

	"home-go/scheduler/optimizer"
)

// GetProfileForMode returns the optimizer profile for a specific dishwasher mode
// Device modes are defined here, but the Profile struct is generic from the optimizer package
func GetProfileForMode(mode Mode) (optimizer.Profile, error) {
	profiles := map[Mode]optimizer.Profile{
		ModeEco: {
			Mode:          string(ModeEco),
			DurationHours: 4,
			// Pre-wash (low), Main wash (max - most important), Dry (medium)
			StageWeights:      []float64{optimizer.WeightLow, optimizer.WeightMax, optimizer.WeightMedium},
			PowerKW:           0.8,
			MinSavingsPercent: 5.0,
		},
		ModeAuto: {
			Mode:          string(ModeAuto),
			DurationHours: 3,
			// Pre-wash (low), Main wash (high), Dry (medium)
			StageWeights:      []float64{optimizer.WeightLow, optimizer.WeightHigh, optimizer.WeightMedium},
			PowerKW:           1.2,
			MinSavingsPercent: 5.0,
		},
		ModeAutoQuick: {
			Mode:          string(ModeAutoQuick),
			DurationHours: 2, // Measured: ~70 minutes with VarioDry quick option
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
			PowerKW:           2.0, // Measured: ~2000W during active washing
			MinSavingsPercent: 5.0,
		},
		ModeIntensive: {
			Mode:          string(ModeIntensive),
			DurationHours: 3,
			// All stages use high power
			StageWeights:      []float64{optimizer.WeightMedium, optimizer.WeightMax, optimizer.WeightMedium},
			PowerKW:           1.5,
			MinSavingsPercent: 5.0,
		},
		ModeQuick: {
			Mode:          string(ModeQuick),
			DurationHours: 1,
			// Pre-wash (medium), Main wash (max - critical), No dry
			StageWeights:      []float64{optimizer.WeightMedium, optimizer.WeightMax, optimizer.WeightIdle},
			PowerKW:           2.0,
			MinSavingsPercent: 3.0, // More lenient for quick mode
		},
	}

	profile, ok := profiles[mode]
	if !ok {
		return optimizer.Profile{}, fmt.Errorf("unknown dishwasher mode: %s", mode)
	}

	return profile, nil
}
