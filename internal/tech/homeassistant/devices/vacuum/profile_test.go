package vacuum

import (
	"testing"
	"time"
)

func TestProfile(t *testing.T) {
	if Profile.TotalDuration != 1*time.Hour {
		t.Fatalf("expected duration 1h, got %s", Profile.TotalDuration)
	}
	if Profile.WindowSize != 12*time.Hour {
		t.Fatalf("expected window 12h, got %s", Profile.WindowSize)
	}
	if Profile.Name == "" {
		t.Fatal("expected profile name to be set")
	}
	if Profile.Description == "" {
		t.Fatal("expected profile description to be set")
	}

	req := Profile.ToOptimizerRequest()
	if req.TotalDuration != Profile.TotalDuration {
		t.Fatalf("expected optimizer duration %s, got %s", Profile.TotalDuration, req.TotalDuration)
	}
	if req.WindowSize != Profile.WindowSize {
		t.Fatalf("expected optimizer window %s, got %s", Profile.WindowSize, req.WindowSize)
	}
}
