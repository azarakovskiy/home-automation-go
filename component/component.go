package component

import ga "saml.dev/gome-assistant"

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
}

// NewBase creates a new Base with common services
func NewBase(service *ga.Service) Base {
	return Base{
		Service: service,
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
