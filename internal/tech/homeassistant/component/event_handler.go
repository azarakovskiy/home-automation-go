package component

import (
	"encoding/json"
	"log"

	"home-go/entities"

	ga "saml.dev/gome-assistant"
)

// TypedEventHandler wraps an event handler with automatic type parsing
// This eliminates boilerplate unmarshaling in each component
type TypedEventHandler[T any] struct {
	eventType string
	handler   func(service *ga.Service, state ga.State, data T)
}

// NewTypedEventHandler creates a new typed event handler
func NewTypedEventHandler[T any](
	eventType string,
	handler func(service *ga.Service, state ga.State, data T),
) *TypedEventHandler[T] {
	return &TypedEventHandler[T]{
		eventType: eventType,
		handler:   handler,
	}
}

// Build creates a ga.EventListener that handles parsing automatically
func (h *TypedEventHandler[T]) Build() ga.EventListener {
	return ga.NewEventListener().
		EventTypes(h.eventType).
		Call(h.handle).
		Build()
}

// handle is the internal callback that performs unmarshaling
func (h *TypedEventHandler[T]) handle(service *ga.Service, state ga.State, event ga.EventData) {
	var e entities.HASS[T]
	if err := json.Unmarshal(event.RawEventJSON, &e); err != nil {
		log.Printf("ERROR: Failed to unmarshal event %s: %v", h.eventType, err)
		return
	}

	// Call the typed handler with parsed data
	h.handler(service, state, e.Event.Data)
}
