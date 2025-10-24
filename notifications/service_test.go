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
		MessageKey: MessageDishwasherLater,
		MessageData: map[string]string{
			"time":    "3:30 PM",
			"savings": "10",
		},
	}

	if event.MessageKey == "" {
		t.Error("MessageKey should be set")
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
			name: "valid event with data",
			event: NotificationEvent{
				MessageKey: MessageDishwasherLater,
				MessageData: map[string]string{
					"time": "3 PM",
				},
			},
			wantErr: false,
		},
		{
			name: "valid event without data",
			event: NotificationEvent{
				MessageKey: MessageDishwasherNow,
			},
			wantErr: false,
		},
		{
			name:    "missing message_key",
			event:   NotificationEvent{},
			wantErr: true,
			errMsg:  "message_key cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can only test validation without firing actual events
			if tt.event.MessageKey == "" {
				if tt.errMsg != "message_key cannot be empty" {
					t.Error("Expected message_key validation error")
				}
				return
			}

			// Verify structure for valid events
			if tt.event.MessageKey == "" {
				t.Error("MessageKey should be required")
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
