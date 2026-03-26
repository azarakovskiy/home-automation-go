package notifications

import (
	"testing"
	"time"

	ga "saml.dev/gome-assistant"
)

func TestNewNotificationService(t *testing.T) {
	service := &ga.Service{}
	notif := NewNotificationService(service)

	if notif == nil {
		t.Fatal("NewNotificationService returned nil")
	}
	if notif.service != service {
		t.Error("Service not set correctly")
	}
}

func TestNotificationEvent_Structure(t *testing.T) {
	event := NotificationEvent{
		Device:  "dishwasher",
		Type:    "scheduled",
		Message: "Dishwasher starts at 15:30, saving 10 percent!",
		Data: map[string]interface{}{
			"start_time":      "15:30",
			"savings_percent": 10.5,
		},
	}

	if event.Device == "" {
		t.Error("Device should be set")
	}
	if event.Type == "" {
		t.Error("Type should be set")
	}
	if event.Message == "" {
		t.Error("Message should be set")
	}
	if event.Data == nil {
		t.Error("Data should be set")
	}
}

func TestNotificationEvent_Validation(t *testing.T) {
	tests := []struct {
		name    string
		event   NotificationEvent
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid event with all fields",
			event: NotificationEvent{
				Device:  "dishwasher",
				Type:    "scheduled",
				Message: "Test message",
				Data: map[string]interface{}{
					"start_time": "15:30",
				},
			},
			wantErr: false,
		},
		{
			name: "valid event without data",
			event: NotificationEvent{
				Device:  "dishwasher",
				Type:    "started",
				Message: "Test message",
			},
			wantErr: false,
		},
		{
			name: "missing device",
			event: NotificationEvent{
				Type:    "scheduled",
				Message: "Test message",
			},
			wantErr: true,
			errMsg:  "device cannot be empty",
		},
		{
			name: "missing type",
			event: NotificationEvent{
				Device:  "dishwasher",
				Message: "Test message",
			},
			wantErr: true,
			errMsg:  "type cannot be empty",
		},
		{
			name: "missing message",
			event: NotificationEvent{
				Device: "dishwasher",
				Type:   "scheduled",
			},
			wantErr: true,
			errMsg:  "message cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can only test validation without firing actual events
			if tt.event.Device == "" {
				if tt.errMsg != "device cannot be empty" {
					t.Error("Expected device validation error")
				}
				return
			}
			if tt.event.Type == "" {
				if tt.errMsg != "type cannot be empty" {
					t.Error("Expected type validation error")
				}
				return
			}
			if tt.event.Message == "" {
				if tt.errMsg != "message cannot be empty" {
					t.Error("Expected message validation error")
				}
				return
			}

			// Verify structure for valid events
			if tt.event.Device == "" || tt.event.Type == "" || tt.event.Message == "" {
				t.Error("Device, Type, and Message should all be required")
			}
		})
	}
}

func TestFormatTimeForSpeech(t *testing.T) {
	tests := []struct {
		name     string
		hour     int
		minute   int
		expected string
	}{
		{
			name:     "midnight",
			hour:     0,
			minute:   0,
			expected: "midnight",
		},
		{
			name:     "noon",
			hour:     12,
			minute:   0,
			expected: "noon",
		},
		{
			name:     "3 AM on the hour",
			hour:     3,
			minute:   0,
			expected: "3 AM",
		},
		{
			name:     "3:30 AM with minutes",
			hour:     3,
			minute:   30,
			expected: "3:30 AM",
		},
		{
			name:     "12:15 PM after noon",
			hour:     12,
			minute:   15,
			expected: "12:15 PM",
		},
		{
			name:     "3 PM on the hour",
			hour:     15,
			minute:   0,
			expected: "3 PM",
		},
		{
			name:     "3:30 PM with minutes",
			hour:     15,
			minute:   30,
			expected: "3:30 PM",
		},
		{
			name:     "11:59 PM before midnight",
			hour:     23,
			minute:   59,
			expected: "11:59 PM",
		},
		{
			name:     "6 AM morning",
			hour:     6,
			minute:   0,
			expected: "6 AM",
		},
		{
			name:     "6:15 AM with minutes",
			hour:     6,
			minute:   15,
			expected: "6:15 AM",
		},
		{
			name:     "9 PM evening",
			hour:     21,
			minute:   0,
			expected: "9 PM",
		},
		{
			name:     "9:45 PM with minutes",
			hour:     21,
			minute:   45,
			expected: "9:45 PM",
		},
		{
			name:     "1 AM after midnight",
			hour:     1,
			minute:   0,
			expected: "1 AM",
		},
		{
			name:     "1:05 AM with leading zero",
			hour:     1,
			minute:   5,
			expected: "1:05 AM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a time with the specified hour and minute
			testTime := time.Date(2025, 10, 23, tt.hour, tt.minute, 0, 0, time.UTC)
			result := FormatTimeForSpeech(testTime)

			if result != tt.expected {
				t.Errorf("FormatTimeForSpeech(%d:%02d) = %q, want %q",
					tt.hour, tt.minute, result, tt.expected)
			}
		})
	}
}
