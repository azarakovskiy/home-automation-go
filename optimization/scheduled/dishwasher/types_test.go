package dishwasher

import (
	"testing"
)

func TestMode_Constants(t *testing.T) {
	modes := []Mode{
		ModeEco,
		ModeAuto,
		ModeAutoQuick,
		ModeIntensive,
		ModeQuick,
	}

	// Verify all modes have string representations
	for _, mode := range modes {
		if string(mode) == "" {
			t.Errorf("Mode has empty string representation")
		}
	}

	// Verify modes are unique
	seen := make(map[Mode]bool)
	for _, mode := range modes {
		if seen[mode] {
			t.Errorf("Duplicate mode: %s", mode)
		}
		seen[mode] = true
	}

	// Verify string values match expected
	tests := []struct {
		mode     Mode
		expected string
	}{
		{ModeEco, "eco"},
		{ModeAuto, "auto"},
		{ModeAutoQuick, "auto_quick"},
		{ModeIntensive, "intensive"},
		{ModeQuick, "quick"},
	}

	for _, tt := range tests {
		if string(tt.mode) != tt.expected {
			t.Errorf("Mode %v string = %s, want %s", tt.mode, string(tt.mode), tt.expected)
		}
	}
}
