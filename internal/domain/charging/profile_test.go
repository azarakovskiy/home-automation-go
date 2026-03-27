package charging

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
