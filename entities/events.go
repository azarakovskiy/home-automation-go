package entities

// HASS is the generic wrapper for all Home Assistant events
type HASS[T any] struct {
	Type  string          `json:"type"`
	Event HAInnerEvent[T] `json:"event"`
	ID    int             `json:"id"`
}

// HAInnerEvent is the inner event structure from Home Assistant
type HAInnerEvent[T any] struct {
	EventType string `json:"event_type"`
	Data      T      `json:"data"`
	Origin    string `json:"origin"`
	TimeFired string `json:"time_fired"`
	Context   any    `json:"context"`
}
