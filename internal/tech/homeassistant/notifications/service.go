package notifications

import (
	"fmt"
	"log"
	"maps"
	"time"

	"home-go/internal/tech/homeassistant/entities"

	ga "saml.dev/gome-assistant"
)

// Event describes a user-facing notification delivered via Home Assistant.
type Event struct {
	Device  string         `json:"device"`
	Type    string         `json:"type"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
}

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

// Notify fires a custom event that can be handled by Home Assistant automations
// Event type will be: custom_notify
func (n *NotificationService) Notify(event Event) error {
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

// FormatTimeForSpeech converts a time to a natural spoken format.
func FormatTimeForSpeech(t time.Time) string {
	hour := t.Hour()
	minute := t.Minute()

	if hour == 0 && minute == 0 {
		return "midnight"
	}
	if hour == 12 && minute == 0 {
		return "noon"
	}

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

	if minute == 0 {
		return fmt.Sprintf("%d %s", displayHour, period)
	}

	return fmt.Sprintf("%d:%02d %s", displayHour, minute, period)
}
