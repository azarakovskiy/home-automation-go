package notifications

import (
	"fmt"
	"log"
	"os"
	"time"

	"home-go/entities"

	ga "saml.dev/gome-assistant"
)

// Message keys for voice transformations
const (
	MessageDishwasherNow   = "dishwasher_now"   // Start dishwasher immediately
	MessageDishwasherLater = "dishwasher_later" // Dishwasher scheduled for later
)

// NotificationService provides notifications via custom events
// These events can be handled by Home Assistant automations
type NotificationService struct {
	service  *ga.Service
	useTerry bool // Enable Terry Crews voice transformation
}

// NewNotificationService creates a new notification service
func NewNotificationService(service *ga.Service) *NotificationService {
	// Check if Terry voice is enabled via environment variable (default: enabled)
	useTerry := true
	if terryEnv := os.Getenv("TERRY_VOICE"); terryEnv == "false" {
		useTerry = false
	}

	return &NotificationService{
		service:  service,
		useTerry: useTerry,
	}
}

// SetTerryVoice enables or disables Terry Crews voice transformation
func (n *NotificationService) SetTerryVoice(enabled bool) {
	n.useTerry = enabled
}

// NotificationEvent contains data for notification events
// Device constructs the message template, NotificationService transforms it based on voice settings
type NotificationEvent struct {
	MessageKey  string            `json:"message_key"`  // Template key for voice transformation (e.g., "dishwasher_later")
	MessageData map[string]string `json:"message_data"` // Data to substitute in template (e.g., {"time": "3 PM", "savings": "15"})
}

// Notify fires a custom event that can be handled by Home Assistant automations
// Event type will be: custom_notify
// If Terry voice is enabled, transforms the message using Terry translations
func (n *NotificationService) Notify(event NotificationEvent) error {
	if event.MessageKey == "" {
		return fmt.Errorf("message_key cannot be empty")
	}

	// Transform message using Terry voice if enabled
	message := event.MessageKey // Default to key if no transformation
	if n.useTerry {
		message = GetTerryMessageWithData(event.MessageKey, event.MessageData)
	}

	log.Printf("Firing notification event: message=%s", message)

	// Fire custom event using the Event service
	eventData := map[string]any{
		"message": message,
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
