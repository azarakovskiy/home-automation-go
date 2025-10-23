package dishwasher

import (
	"encoding/json"
	"testing"

	"home-go/entities"
	"home-go/scheduler"
)

func TestScheduleRequest_Unmarshal(t *testing.T) {
	jsonData := `{
		"type": "event",
		"event": {
			"event_type": "event.custom_scheduled_start",
			"data": {
				"device": "dishwasher",
				"mode": "auto",
				"max_delay_hours": 8
			},
			"origin": "LOCAL",
			"time_fired": "2025-10-22T10:00:00+00:00",
			"context": {}
		},
		"id": 1
	}`

	var event entities.HASS[scheduler.ScheduleRequest]
	err := json.Unmarshal([]byte(jsonData), &event)

	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if event.Type != "event" {
		t.Errorf("Type = %s, want 'event'", event.Type)
	}

	if event.Event.EventType != "event.custom_scheduled_start" {
		t.Errorf("EventType = %s, want 'event.custom_scheduled_start'", event.Event.EventType)
	}

	data := event.Event.Data
	if data.Device != "dishwasher" {
		t.Errorf("Device = %s, want 'dishwasher'", data.Device)
	}

	if data.Mode != "auto" {
		t.Errorf("Mode = %s, want 'auto'", data.Mode)
	}

	if data.MaxDelayHours != 8 {
		t.Errorf("MaxDelayHours = %d, want 8", data.MaxDelayHours)
	}
}

func TestScheduleRequest_DifferentModes(t *testing.T) {
	modes := []string{"eco", "auto", "intensive", "quick"}

	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			jsonData := `{
				"type": "event",
				"event": {
					"event_type": "event.custom_scheduled_start",
					"data": {
						"device": "dishwasher",
						"mode": "` + mode + `",
						"max_delay_hours": 6
					},
					"origin": "LOCAL",
					"time_fired": "2025-10-22T10:00:00+00:00",
					"context": {}
				},
				"id": 1
			}`

			var event entities.HASS[scheduler.ScheduleRequest]
			err := json.Unmarshal([]byte(jsonData), &event)

			if err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if string(event.Event.Data.Mode) != mode {
				t.Errorf("Mode = %s, want %s", event.Event.Data.Mode, mode)
			}
		})
	}
}

func TestMode_Constants(t *testing.T) {
	modes := []Mode{
		ModeEco,
		ModeAuto,
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
		{ModeIntensive, "intensive"},
		{ModeQuick, "quick"},
	}

	for _, tt := range tests {
		if string(tt.mode) != tt.expected {
			t.Errorf("Mode %v string = %s, want %s", tt.mode, string(tt.mode), tt.expected)
		}
	}
}
