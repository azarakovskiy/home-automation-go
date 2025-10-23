package dishwasher

import (
	"testing"
	"time"
)

func TestGetProfileForMode_AllModes(t *testing.T) {
	tests := []struct {
		mode         Mode
		wantDuration time.Duration
		wantStages   int
		wantPowerMin float64
		wantPowerMax float64
	}{
		{
			mode:         ModeEco,
			wantDuration: 4 * time.Hour,
			wantStages:   3,
			wantPowerMin: 0.7,
			wantPowerMax: 0.9,
		},
		{
			mode:         ModeAuto,
			wantDuration: 137 * time.Minute, // Measured: exactly 137 minutes
			wantStages:   7,                 // Updated: 7 stages from measured data
			wantPowerMin: 1.8,               // Updated: measured ~2000W
			wantPowerMax: 2.1,
		},
		{
			mode:         ModeAutoQuick,
			wantDuration: 70 * time.Minute, // Measured: ~70 minutes
			wantStages:   13,               // Measured: 13 distinct stages from power graph
			wantPowerMin: 1.8,              // Real measured: ~2000W
			wantPowerMax: 2.1,
		},
		{
			mode:         ModeIntensive,
			wantDuration: 3 * time.Hour,
			wantStages:   3,
			wantPowerMin: 1.4,
			wantPowerMax: 1.6,
		},
		{
			mode:         ModeQuick,
			wantDuration: 1 * time.Hour,
			wantStages:   3,
			wantPowerMin: 1.8, // Real measured: ~2000W
			wantPowerMax: 2.1,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			profile, err := GetProfileForMode(tt.mode)
			if err != nil {
				t.Fatalf("GetProfileForMode(%s) failed: %v", tt.mode, err)
			}

			if profile.GetDuration() != tt.wantDuration {
				t.Errorf("Duration = %s, want %s", profile.GetDuration(), tt.wantDuration)
			}

			weights := profile.GetStageWeights()
			if len(weights) != tt.wantStages {
				t.Errorf("Stage count = %d, want %d", len(weights), tt.wantStages)
			}

			// Verify weights are valid (importance weights, don't need to sum to 1.0)
			sum := 0.0
			for _, w := range weights {
				sum += w
			}
			// Just verify we have some weights and they're reasonable
			if sum < 0.5 {
				t.Errorf("Weights sum = %.2f, seems too low", sum)
			}

			power := profile.GetPowerKW()
			if power < tt.wantPowerMin || power > tt.wantPowerMax {
				t.Errorf("Power = %.2f, want between %.2f and %.2f", power, tt.wantPowerMin, tt.wantPowerMax)
			}

			if profile.GetMode() != string(tt.mode) {
				t.Errorf("Mode = %s, want %s", profile.GetMode(), tt.mode)
			}
		})
	}
}

func TestGetProfileForMode_UnknownMode(t *testing.T) {
	_, err := GetProfileForMode("unknown_mode")
	if err == nil {
		t.Error("Expected error for unknown mode, got nil")
	}
}

func TestProfile_ImplementsDeviceProfile(t *testing.T) {
	profile, err := GetProfileForMode(ModeAuto)
	if err != nil {
		t.Fatalf("Failed to get profile: %v", err)
	}

	// Verify interface methods work
	if profile.GetDuration() <= 0 {
		t.Error("Duration should be positive")
	}

	if len(profile.GetStageWeights()) == 0 {
		t.Error("Should have stage weights")
	}

	if profile.GetPowerKW() <= 0 {
		t.Error("Power should be positive")
	}

	if profile.GetMode() == "" {
		t.Error("Mode should not be empty")
	}
}

func TestProfile_StageWeights(t *testing.T) {
	tests := []struct {
		mode               Mode
		expectedStageCount int
	}{
		{ModeEco, 3},
		{ModeAuto, 7},       // Updated: 7 stages from measured data
		{ModeAutoQuick, 13}, // 13 stages based on measured power pattern
		{ModeIntensive, 3},
		{ModeQuick, 3},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			profile, err := GetProfileForMode(tt.mode)
			if err != nil {
				t.Fatalf("GetProfileForMode failed: %v", err)
			}

			weights := profile.GetStageWeights()
			if len(weights) != tt.expectedStageCount {
				t.Errorf("Stage count = %d, want %d", len(weights), tt.expectedStageCount)
			}

			// Verify weights are reasonable (none negative, all <= 1.0)
			for i, w := range weights {
				if w < 0 || w > 1.0 {
					t.Errorf("Stage %d weight = %.2f, should be between 0 and 1.0", i, w)
				}
			}
		})
	}
}

func TestProfile_EcoVsIntensive(t *testing.T) {
	eco, err := GetProfileForMode(ModeEco)
	if err != nil {
		t.Fatalf("Failed to get eco profile: %v", err)
	}

	intensive, err := GetProfileForMode(ModeIntensive)
	if err != nil {
		t.Fatalf("Failed to get intensive profile: %v", err)
	}

	// Eco should take longer
	if eco.GetDuration() <= intensive.GetDuration() {
		t.Errorf("Eco duration (%s) should be longer than intensive (%s)",
			eco.GetDuration(), intensive.GetDuration())
	}

	// Intensive should use more power
	if intensive.GetPowerKW() <= eco.GetPowerKW() {
		t.Errorf("Intensive power (%.2f) should be higher than eco (%.2f)",
			intensive.GetPowerKW(), eco.GetPowerKW())
	}
}
