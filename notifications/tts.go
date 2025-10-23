package notifications

import (
	"fmt"
	"log"

	"home-go/entities"

	ga "saml.dev/gome-assistant"
)

// TTSService provides text-to-speech announcements to media players
type TTSService struct {
	service *ga.Service
}

// NewTTSService creates a new TTS service
func NewTTSService(service *ga.Service) *TTSService {
	return &TTSService{
		service: service,
	}
}

// AnnouncementConfig configures a TTS announcement
type AnnouncementConfig struct {
	Message           string
	MediaPlayerEntity string
	TTSEntity         string
	Cache             bool
	IgnoreErrors      bool // If true, log errors instead of returning them
}

// DefaultConfig returns a sensible default configuration
// Using Living Room Google Home as default speaker and Piper as TTS engine
func DefaultConfig() AnnouncementConfig {
	return AnnouncementConfig{
		MediaPlayerEntity: entities.MediaPlayer.LivingRoomGoogleHome,
		TTSEntity:         entities.Tts.Piper,
		Cache:             true,
		IgnoreErrors:      true, // Announcements shouldn't break functionality
	}
}

// Announce sends a TTS announcement to the specified media player
func (t *TTSService) Announce(config AnnouncementConfig) error {
	if config.Message == "" {
		return fmt.Errorf("message cannot be empty")
	}

	// Validate entities are set
	if config.MediaPlayerEntity == "" {
		config.MediaPlayerEntity = entities.MediaPlayer.LivingRoomGoogleHome
	}
	if config.TTSEntity == "" {
		config.TTSEntity = entities.Tts.Piper
	}

	log.Printf("TTS: Announcing via %s: %s", config.MediaPlayerEntity, config.Message)

	// Call TTS service using the TTS domain
	// The gome-assistant library generates a Speak method for TTS entities
	// We need to pass the entity ID and service data
	serviceData := map[string]any{
		"message":                config.Message,
		"media_player_entity_id": config.MediaPlayerEntity,
		"cache":                  config.Cache,
	}

	// Use HomeAssistant.TurnOn with the TTS entity to trigger speak action
	// This is a workaround since TTS domain methods may not be directly exposed
	err := t.service.HomeAssistant.TurnOn(config.TTSEntity, serviceData)
	if err != nil {
		if config.IgnoreErrors {
			log.Printf("WARNING: TTS announcement failed (ignored): %v", err)
			return nil
		}
		return fmt.Errorf("failed to send TTS announcement: %w", err)
	}

	return nil
}

// AnnounceToDefault sends a TTS announcement using default configuration
func (t *TTSService) AnnounceToDefault(message string) error {
	config := DefaultConfig()
	config.Message = message
	return t.Announce(config)
}

// AnnounceToAllSpeakers sends the same announcement to multiple speakers
func (t *TTSService) AnnounceToAllSpeakers(message string, mediaPlayers []string) error {
	config := DefaultConfig()
	config.Message = message

	var lastErr error
	for _, player := range mediaPlayers {
		config.MediaPlayerEntity = player
		if err := t.Announce(config); err != nil {
			lastErr = err
			log.Printf("WARNING: Failed to announce to %s: %v", player, err)
		}
	}

	return lastErr
}
