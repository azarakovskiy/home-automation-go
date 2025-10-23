package component

import (
	"fmt"

	"home-go/entities"

	ga "saml.dev/gome-assistant"
)

// Component is a generic interface for any self-contained component.
// Each component (scheduler, lighting, security, etc.) implements this interface
// to declare its listener needs.
type Component interface {
	// EventListeners returns the event listeners this component needs
	EventListeners() []ga.EventListener

	// EntityListeners returns the entity state listeners this component needs
	EntityListeners() []ga.EntityListener

	// Schedules returns the daily schedules this component needs
	Schedules() []ga.DailySchedule

	// Intervals returns the periodic intervals this component needs
	Intervals() []ga.Interval
}

// Base provides default empty implementations of Component interface
// and common services that components typically need.
// Components can embed this struct and override only the methods they need.
//
// Example:
//
//	type MyComponent struct {
//	    component.Base  // Embed to get defaults and common services
//	}
//
//	func New(service *ga.Service) *MyComponent {
//	    return &MyComponent{
//	        Base: component.NewBase(service),
//	    }
//	}
//
//	// Only override what you need
//	func (c *MyComponent) EventListeners() []ga.EventListener {
//	    return []ga.EventListener{...}
//	}
type Base struct {
	Service *ga.Service
	State   ga.State
}

// NewBase creates a new Base with common services
func NewBase(service *ga.Service) Base {
	return Base{
		Service: service,
		State:   nil, // Will be set by component constructor if needed
	}
}

// EventListeners returns an empty slice by default
func (b Base) EventListeners() []ga.EventListener {
	return []ga.EventListener{}
}

// EntityListeners returns an empty slice by default
func (b Base) EntityListeners() []ga.EntityListener {
	return []ga.EntityListener{}
}

// Schedules returns an empty slice by default
func (b Base) Schedules() []ga.DailySchedule {
	return []ga.DailySchedule{}
}

// Intervals returns an empty slice by default
func (b Base) Intervals() []ga.Interval {
	return []ga.Interval{}
}

// IsNightMode checks if the daytime mode is currently "Night"
// This is useful for components that want to adjust behavior based on time of day
func (b Base) IsNightMode() (bool, error) {
	if b.State == nil {
		return false, fmt.Errorf("state not initialized in component")
	}

	state, err := b.State.Get(entities.InputSelect.DaytimeMode)
	if err != nil {
		return false, fmt.Errorf("failed to get daytime mode: %w", err)
	}

	// The state value is directly in state.State for input_select
	return state.State == "Night", nil
}
