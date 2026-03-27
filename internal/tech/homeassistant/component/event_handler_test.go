package component

import (
	"encoding/json"
	"testing"

	"home-go/internal/domain/scheduler"
	"home-go/internal/tech/homeassistant/entities"

	ga "saml.dev/gome-assistant"
)

// TestTypedEventHandler_ParsesAndCallsHandler verifies that TypedEventHandler
// correctly unmarshals the event and calls the handler with typed data
func TestTypedEventHandler_ParsesAndCallsHandler(t *testing.T) {
	// Arrange
	eventType := "event.test_schedule"

	// Create test event JSON
	testEvent := entities.HASS[scheduler.ScheduleRequest]{
		Type: "result",
		Event: entities.HAInnerEvent[scheduler.ScheduleRequest]{
			EventType: eventType,
			Data: scheduler.ScheduleRequest{
				Device:        "test_device",
				Mode:          "test_mode",
				MaxDelayHours: 8,
			},
			Origin:    "LOCAL",
			TimeFired: "2024-01-15T10:00:00Z",
		},
		ID: 123,
	}

	eventJSON, err := json.Marshal(testEvent)
	if err != nil {
		t.Fatalf("Failed to marshal test event: %v", err)
	}

	// Track if handler was called with correct data
	var receivedRequest *scheduler.ScheduleRequest
	handlerCalled := false

	handler := func(service *ga.Service, state ga.State, request scheduler.ScheduleRequest) {
		handlerCalled = true
		receivedRequest = &request
	}

	// Act
	typedHandler := NewTypedEventHandler(eventType, handler)

	// Simulate event callback
	eventData := ga.EventData{
		RawEventJSON: eventJSON,
	}

	// Call the handler directly (simulating gome-assistant calling it)
	typedHandler.handle(nil, nil, eventData)

	// Assert
	if !handlerCalled {
		t.Error("Handler was not called")
	}

	if receivedRequest == nil {
		t.Fatal("Handler was called but received nil request")
	}

	if receivedRequest.Device != "test_device" {
		t.Errorf("Expected device 'test_device', got '%s'", receivedRequest.Device)
	}

	if receivedRequest.Mode != "test_mode" {
		t.Errorf("Expected mode 'test_mode', got '%s'", receivedRequest.Mode)
	}

	if receivedRequest.MaxDelayHours != 8 {
		t.Errorf("Expected max delay 8 hours, got %d", receivedRequest.MaxDelayHours)
	}
}

// TestTypedEventHandler_HandlesInvalidJSON verifies graceful error handling
func TestTypedEventHandler_HandlesInvalidJSON(t *testing.T) {
	// Arrange
	// Invalid JSON that can't be parsed at all
	invalidJSON := []byte(`{this is not valid json}`)

	handlerCalled := false
	handler := func(service *ga.Service, state ga.State, request scheduler.ScheduleRequest) {
		handlerCalled = true
	}

	typedHandler := NewTypedEventHandler("event.test", handler)

	eventData := ga.EventData{
		RawEventJSON: invalidJSON,
	}

	// Act
	typedHandler.handle(nil, nil, eventData)

	// Assert - handler should NOT be called on invalid JSON
	if handlerCalled {
		t.Error("Handler was called despite invalid JSON")
	}
}

// TestTypedEventHandler_MultipleEventTypes demonstrates using multiple typed handlers
func TestTypedEventHandler_MultipleEventTypes(t *testing.T) {
	type EventA struct {
		ValueA string `json:"value_a"`
	}

	type EventB struct {
		ValueB int `json:"value_b"`
	}

	// Create handlers for different event types
	var receivedA *EventA
	var receivedB *EventB

	handlerA := NewTypedEventHandler("event.type_a", func(s *ga.Service, st ga.State, data EventA) {
		receivedA = &data
	})

	handlerB := NewTypedEventHandler("event.type_b", func(s *ga.Service, st ga.State, data EventB) {
		receivedB = &data
	})

	// Create test events
	eventAJSON, _ := json.Marshal(entities.HASS[EventA]{
		Type: "result",
		Event: entities.HAInnerEvent[EventA]{
			EventType: "event.type_a",
			Data:      EventA{ValueA: "test_string"},
		},
	})

	eventBJSON, _ := json.Marshal(entities.HASS[EventB]{
		Type: "result",
		Event: entities.HAInnerEvent[EventB]{
			EventType: "event.type_b",
			Data:      EventB{ValueB: 42},
		},
	})

	// Act
	handlerA.handle(nil, nil, ga.EventData{RawEventJSON: eventAJSON})
	handlerB.handle(nil, nil, ga.EventData{RawEventJSON: eventBJSON})

	// Assert
	if receivedA == nil || receivedA.ValueA != "test_string" {
		t.Error("Handler A did not receive correct data")
	}

	if receivedB == nil || receivedB.ValueB != 42 {
		t.Error("Handler B did not receive correct data")
	}
}
