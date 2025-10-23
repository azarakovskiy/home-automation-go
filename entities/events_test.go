package entities

import (
	"encoding/json"
	"testing"
)

func TestHASS_Unmarshal(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		verify   func(t *testing.T, data any)
	}{
		{
			name: "generic data",
			jsonData: `{
				"type": "event",
				"event": {
					"event_type": "custom_event",
					"data": {
						"value": "test",
						"count": 42
					},
					"origin": "LOCAL",
					"time_fired": "2025-10-22T10:00:00+00:00",
					"context": {}
				},
				"id": 1
			}`,
			verify: func(t *testing.T, data any) {
				type CustomData struct {
					Value string `json:"value"`
					Count int    `json:"count"`
				}

				var event HASS[CustomData]
				if err := json.Unmarshal([]byte(data.(string)), &event); err != nil {
					t.Fatalf("Failed to unmarshal: %v", err)
				}

				if event.Type != "event" {
					t.Errorf("Type = %s, want 'event'", event.Type)
				}
				if event.Event.EventType != "custom_event" {
					t.Errorf("EventType = %s, want 'custom_event'", event.Event.EventType)
				}
				if event.Event.Data.Value != "test" {
					t.Errorf("Value = %s, want 'test'", event.Event.Data.Value)
				}
				if event.Event.Data.Count != 42 {
					t.Errorf("Count = %d, want 42", event.Event.Data.Count)
				}
			},
		},
		{
			name: "nested structure",
			jsonData: `{
				"type": "event",
				"event": {
					"event_type": "state_changed",
					"data": {
						"device": {
							"id": "abc123",
							"name": "Test Device"
						},
						"value": 3.14
					},
					"origin": "LOCAL",
					"time_fired": "2025-10-22T10:00:00+00:00",
					"context": {}
				},
				"id": 2
			}`,
			verify: func(t *testing.T, data any) {
				type Device struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}
				type ComplexData struct {
					Device Device  `json:"device"`
					Value  float64 `json:"value"`
				}

				var event HASS[ComplexData]
				if err := json.Unmarshal([]byte(data.(string)), &event); err != nil {
					t.Fatalf("Failed to unmarshal: %v", err)
				}

				if event.Event.Data.Device.ID != "abc123" {
					t.Errorf("Device ID = %s, want 'abc123'", event.Event.Data.Device.ID)
				}
				if event.Event.Data.Device.Name != "Test Device" {
					t.Errorf("Device Name = %s, want 'Test Device'", event.Event.Data.Device.Name)
				}
				if event.Event.Data.Value != 3.14 {
					t.Errorf("Value = %.2f, want 3.14", event.Event.Data.Value)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.verify(t, tt.jsonData)
		})
	}
}

func TestHASS_Marshal(t *testing.T) {
	type SimpleData struct {
		Action string `json:"action"`
	}

	event := HASS[SimpleData]{
		Type: "event",
		Event: HAInnerEvent[SimpleData]{
			EventType: "test_event",
			Data: SimpleData{
				Action: "test",
			},
			Origin:    "LOCAL",
			TimeFired: "2025-10-22T10:00:00+00:00",
		},
		ID: 1,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Verify we can unmarshal it back
	var decoded HASS[SimpleData]
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Event.Data.Action != "test" {
		t.Errorf("Action = %s, want 'test'", decoded.Event.Data.Action)
	}
}
