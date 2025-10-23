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
			// Pre-wash: 15%, Main wash: 60%, Dry: 25%
			// Main wash uses most power and is most important for optimization
			StageWeights:      []float64{0.15, 0.60, 0.25},
			PowerKW:           0.8,
			MinSavingsPercent: 5.0, // Delay if at least 5% savings
		},
		ModeAuto: {
			Mode:          string(ModeAuto),
			DurationHours: 3, // Standard Auto mode - not yet measured
			// Pre-wash: 20%, Main wash: 50%, Dry: 30%
			StageWeights:      []float64{0.2, 0.5, 0.3},
			PowerKW:           1.2, // Estimate - lower power, longer time
			MinSavingsPercent: 5.0, // Delay if at least 5% savings
		},
		ModeAutoQuick: {
			Mode:          string(ModeAutoQuick),
			DurationHours: 2, // Measured: ~70 minutes with VarioDry quick option
			// Stage weights based on actual power consumption pattern from graphs (13 stages):
			// High weight = high power consumption stages that should run during cheap electricity
			// Low weight = pauses/drains that don't consume much power
			StageWeights: []float64{
				0.40, // 1. First major heating/wash
				0.50, // 2. Continued heating
				0.50, // 3. Main wash
				0.10, // 4. Pause/drain (low power)
				0.10, // 5. Pause/drain (low power)
				0.50, // 6. Major stage
				0.25, // 7. Medium stage
				0.25, // 8. Medium stage
				0.25, // 9. Medium stage
				0.50, // 10. Major heating stage
				0.50, // 11. Drying stage
				0.25, // 12. Final medium stage
				0.05, // 13. Final small stage
			},
			PowerKW:           2.0, // Measured: ~2000W during active washing
			MinSavingsPercent: 5.0, // Delay if at least 5% savings
		},
		ModeIntensive: {
			Mode:          string(ModeIntensive),
			DurationHours: 3,
			// All stages use high power, more balanced optimization
			StageWeights:      []float64{0.25, 0.50, 0.25},
			PowerKW:           1.5,
			MinSavingsPercent: 5.0, // Delay if at least 5% savings
		},
		ModeQuick: {
			Mode:          string(ModeQuick),
			DurationHours: 1, // Not yet measured - estimate for standalone Quick mode
			// No dry stage, main wash is critical
			StageWeights:      []float64{0.3, 0.7, 0.0},
			PowerKW:           2.0, // Assuming similar power to Auto mode
			MinSavingsPercent: 3.0, // More lenient for quick mode (less overall cost)
		},
	}

	profile, ok := profiles[mode]
	if !ok {
		return optimizer.Profile{}, fmt.Errorf("unknown dishwasher mode: %s", mode)
	}

	return profile, nil
}
