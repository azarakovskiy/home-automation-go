package reminder

import (
	"testing"
	"time"

	"home-go/events"
)

func TestNormalizeMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		oneTime  *bool
		expected ReminderMode
	}{
		{"explicit single", "single", nil, ModeSingle},
		{"alias one-time", "one_time", nil, ModeSingle},
		{"explicit repeating", "repeating", nil, ModeRepeating},
		{"legacy flag true", "", ptrBool(true), ModeSingle},
		{"default repeating", "", nil, ModeRepeating},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeMode(tt.mode, tt.oneTime); got != tt.expected {
				t.Fatalf("expected %s, got %s", tt.expected, got)
			}
		})
	}
}

func TestResolveStartTime(t *testing.T) {
	now := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	delay := 15
	event := events.ReminderCreateEvent{
		InitialDelayMinutes: &delay,
	}

	start, err := resolveStartTime(event, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !start.Equal(now.Add(15 * time.Minute)) {
		t.Fatalf("expected %s, got %s", now.Add(15*time.Minute), start)
	}

	event.InitialDelayMinutes = nil
	event.StartTime = "13:30"
	start, err = resolveStartTime(event, now)
	if err != nil {
		t.Fatalf("unexpected error parsing clock time: %v", err)
	}
	if start.Hour() != 13 || start.Minute() != 30 {
		t.Fatalf("expected 13:30, got %s", start)
	}

	event.StartTime = "invalid"
	if _, err := resolveStartTime(event, now); err == nil {
		t.Fatalf("expected error for invalid start time")
	}
}

func ptrBool(v bool) *bool {
	return &v
}
