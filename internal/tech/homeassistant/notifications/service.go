package notifications

import (
	"fmt"
	"log"
	"maps"
	"time"

	"home-go/entities"

	ga "saml.dev/gome-assistant"
)

// NotificationService provides notifications via custom events
// These events can be handled by Home Assistant automations
type NotificationService struct {
	service *ga.Service
}

// NewNotificationService creates a new notification service
func NewNotificationService(service *ga.Service) *NotificationService {
	return &NotificationService{
		service: service,
	}
}

// NotificationEvent contains data for notification events
// Device constructs the message, HASS automation just plays it via TTS
type NotificationEvent struct {
	Device  string                 `json:"device"`         // e.g., "dishwasher", "heating"
	Type    string                 `json:"type"`           // e.g., "scheduled", "started", "completed"
	Message string                 `json:"message"`        // The message to announce
	Data    map[string]interface{} `json:"data,omitempty"` // Optional additional data
}

// Notify fires a custom event that can be handled by Home Assistant automations
// Event type will be: custom_notify
func (n *NotificationService) Notify(event NotificationEvent) error {
	if event.Device == "" {
		return fmt.Errorf("device cannot be empty")
	}
	if event.Type == "" {
		return fmt.Errorf("type cannot be empty")
	}
	if event.Message == "" {
		return fmt.Errorf("message cannot be empty")
	}

	log.Printf("Firing notification event: device=%s, type=%s, message=%s",
		event.Device, event.Type, event.Message)

	// Fire custom event using the Event service
	eventData := map[string]any{
		"device":  event.Device,
		"type":    event.Type,
		"message": event.Message,
	}

	// Merge additional data
	if event.Data != nil {
		maps.Copy(eventData, event.Data)
	}

	err := n.service.Event.Fire(entities.CustomEvents.Notify, eventData)
	if err != nil {
		return fmt.Errorf("failed to fire notification event: %w", err)
	}

	return nil
}

// FormatTimeForSpeech converts a time to a natural spoken format
// Examples: "3 PM", "3:30 PM", "noon", "midnight"
func FormatTimeForSpeech(t time.Time) string {
	hour := t.Hour()
	minute := t.Minute()

	// Special cases
	if hour == 0 && minute == 0 {
		return "midnight"
	}
	if hour == 12 && minute == 0 {
		return "noon"
	}

	// Convert to 12-hour format
	period := "AM"
	displayHour := hour
	if hour >= 12 {
		period = "PM"
		if hour > 12 {
			displayHour = hour - 12
		}
	}
	if displayHour == 0 {
		displayHour = 12
	}

	// Format based on whether we have minutes
	if minute == 0 {
		return fmt.Sprintf("%d %s", displayHour, period)
	}
	return fmt.Sprintf("%d:%02d %s", displayHour, minute, period)
}
