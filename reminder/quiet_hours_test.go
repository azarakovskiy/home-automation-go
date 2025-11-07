package reminder

import (
	"testing"
	"time"
)

func TestQuietHoursIsQuiet(t *testing.T) {
	config := QuietHoursConfig{Enabled: true, Start: "22:00", End: "07:00"}

	check := func(hour, minute int, want bool) {
		moment := time.Date(2024, 1, 1, hour, minute, 0, 0, time.UTC)
		if got := config.isQuiet(moment); got != want {
			t.Fatalf("%02d:%02d expected %v, got %v", hour, minute, want, got)
		}
	}

	check(21, 59, false)
	check(22, 0, true)
	check(2, 30, true)
	check(7, 0, false)
	check(8, 0, false)
}

func TestQuietHoursNextWindowEnd(t *testing.T) {
	config := QuietHoursConfig{Enabled: true, Start: "22:00", End: "07:00"}
	start := time.Date(2024, 1, 1, 23, 0, 0, 0, time.UTC)

	end := config.nextWindowEnd(start)
	expected := time.Date(2024, 1, 2, 7, 0, 0, 0, time.UTC)
	if !end.Equal(expected) {
		t.Fatalf("expected %s, got %s", expected, end)
	}
}
