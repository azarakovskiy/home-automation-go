package continuous

import (
	"testing"
	"time"
)

func TestChargingProfile_ToOptimizerRequest(t *testing.T) {
	profile := ChargingProfile{
		Name:          "Test",
		TotalDuration: 4 * time.Hour,
		WindowSize:    8 * time.Hour,
		Description:   "Test profile",
	}

	req := profile.ToOptimizerRequest()

	if req.TotalDuration != 4*time.Hour {
		t.Errorf("Expected TotalDuration 4h, got %s", req.TotalDuration)
	}
	if req.WindowSize != 8*time.Hour {
		t.Errorf("Expected WindowSize 8h, got %s", req.WindowSize)
	}
}

func TestPredefinedProfiles(t *testing.T) {
	tests := []struct {
		name            string
		profile         ChargingProfile
		expectedDur     time.Duration
		expectedWindow  time.Duration
		expectedNameSet bool
	}{
		{
			name:            "Laptop Profile",
			profile:         LaptopProfile,
			expectedDur:     6 * time.Hour,
			expectedWindow:  12 * time.Hour, // 12h window for finding cheap charging slots
			expectedNameSet: true,
		},
		{
			name:            "Vacuum Profile",
			profile:         VacuumProfile,
			expectedDur:     1 * time.Hour,
			expectedWindow:  12 * time.Hour,
			expectedNameSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.profile.TotalDuration != tt.expectedDur {
				t.Errorf("Expected duration %s, got %s", tt.expectedDur, tt.profile.TotalDuration)
			}
			if tt.profile.WindowSize != tt.expectedWindow {
				t.Errorf("Expected window %s, got %s", tt.expectedWindow, tt.profile.WindowSize)
			}
			if tt.expectedNameSet && tt.profile.Name == "" {
				t.Error("Expected profile name to be set")
			}
			if tt.profile.Description == "" {
				t.Error("Expected profile description to be set")
			}

			// Verify ToOptimizerRequest works
			req := tt.profile.ToOptimizerRequest()
			if req.TotalDuration != tt.profile.TotalDuration {
				t.Errorf("ToOptimizerRequest: Expected TotalDuration %s, got %s",
					tt.profile.TotalDuration, req.TotalDuration)
			}
			if req.WindowSize != tt.profile.WindowSize {
				t.Errorf("ToOptimizerRequest: Expected WindowSize %s, got %s",
					tt.profile.WindowSize, req.WindowSize)
			}
		})
	}
}

func TestProfiles_LogicalConstraints(t *testing.T) {
	profiles := []ChargingProfile{
		LaptopProfile,
		VacuumProfile,
	}

	for _, profile := range profiles {
		t.Run(profile.Name, func(t *testing.T) {
			if profile.TotalDuration <= 0 {
				t.Errorf("Profile %s has non-positive duration: %s", profile.Name, profile.TotalDuration)
			}
			if profile.WindowSize <= 0 {
				t.Errorf("Profile %s has non-positive window: %s", profile.Name, profile.WindowSize)
			}
			if profile.TotalDuration > profile.WindowSize {
				t.Errorf("Profile %s has duration (%s) > window (%s)",
					profile.Name, profile.TotalDuration, profile.WindowSize)
			}
		})
	}
}
