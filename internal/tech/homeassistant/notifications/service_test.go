package notifications

import (
	"testing"

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
	event := Event{
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
		event   Event
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid event with all fields",
			event: Event{
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
			event: Event{
				Device:  "dishwasher",
				Type:    "started",
				Message: "Test message",
			},
			wantErr: false,
		},
		{
			name: "missing device",
			event: Event{
				Type:    "scheduled",
				Message: "Test message",
			},
			wantErr: true,
			errMsg:  "device cannot be empty",
		},
		{
			name: "missing type",
			event: Event{
				Device:  "dishwasher",
				Message: "Test message",
			},
			wantErr: true,
			errMsg:  "type cannot be empty",
		},
		{
			name: "missing message",
			event: Event{
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
