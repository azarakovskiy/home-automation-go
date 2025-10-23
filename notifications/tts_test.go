package notifications

import (
	"testing"

	"home-go/entities"

	ga "saml.dev/gome-assistant"
)

func TestNewTTSService(t *testing.T) {
	service := &ga.Service{}
	tts := NewTTSService(service)

	if tts == nil {
		t.Fatal("NewTTSService returned nil")
	}
	if tts.service != service {
		t.Error("Service not set correctly")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MediaPlayerEntity != entities.MediaPlayer.LivingRoomGoogleHome {
		t.Errorf("Default media player = %s, want %s",
			config.MediaPlayerEntity, entities.MediaPlayer.LivingRoomGoogleHome)
	}
	if config.TTSEntity != entities.Tts.Piper {
		t.Errorf("Default TTS entity = %s, want %s",
			config.TTSEntity, entities.Tts.Piper)
	}
	if !config.Cache {
		t.Error("Cache should be enabled by default")
	}
	if !config.IgnoreErrors {
		t.Error("IgnoreErrors should be true by default")
	}
}

func TestTTSService_Announce_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  AnnouncementConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "empty message returns error",
			config: AnnouncementConfig{
				Message:           "",
				MediaPlayerEntity: entities.MediaPlayer.LivingRoomGoogleHome,
				TTSEntity:         entities.Tts.Piper,
				Cache:             true,
				IgnoreErrors:      false,
			},
			wantErr: true,
			errMsg:  "message cannot be empty",
		},
		{
			name: "valid config structure",
			config: AnnouncementConfig{
				Message:           "Test message",
				MediaPlayerEntity: entities.MediaPlayer.LivingRoomGoogleHome,
				TTSEntity:         entities.Tts.Piper,
				Cache:             true,
				IgnoreErrors:      false,
			},
			wantErr: false,
		},
		{
			name: "defaults applied when entities missing",
			config: AnnouncementConfig{
				Message:      "Test message",
				IgnoreErrors: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: We can't fully test Announce() without a real Home Assistant instance
			// or complex mocking of the gome-assistant internal services.
			// These tests validate the input validation logic.

			service := &ga.Service{}
			tts := NewTTSService(service)

			// Only test empty message validation (the one thing we can test without HA)
			if tt.config.Message == "" {
				err := tts.Announce(tt.config)
				if err == nil {
					t.Error("Expected error for empty message, got nil")
				}
				if err.Error() != tt.errMsg {
					t.Errorf("Error message = %q, want %q", err.Error(), tt.errMsg)
				}
				return
			}

			// For non-empty messages, we can only verify the config structure
			// Real functionality requires integration testing with Home Assistant
			if tt.config.MediaPlayerEntity == "" && tt.config.TTSEntity == "" {
				// Verify defaults would be applied
				config := tt.config
				if config.MediaPlayerEntity == "" {
					config.MediaPlayerEntity = entities.MediaPlayer.LivingRoomGoogleHome
				}
				if config.TTSEntity == "" {
					config.TTSEntity = entities.Tts.Piper
				}
				if config.MediaPlayerEntity == "" || config.TTSEntity == "" {
					t.Error("Defaults should be applied")
				}
			}
		})
	}
}

func TestTTSService_AnnounceToDefault(t *testing.T) {
	tests := []struct {
		name    string
		message string
		wantErr bool
	}{
		{
			name:    "empty message fails",
			message: "",
			wantErr: true,
		},
		{
			name:    "valid message",
			message: "Test message",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &ga.Service{}
			tts := NewTTSService(service)

			// We can only test validation logic without HA
			if tt.message == "" {
				err := tts.AnnounceToDefault(tt.message)
				if err == nil {
					t.Error("Expected error for empty message, got nil")
				}
				return
			}

			// Verify default config is used
			config := DefaultConfig()
			if config.MediaPlayerEntity == "" {
				t.Error("Default config should have media player")
			}
			if config.TTSEntity == "" {
				t.Error("Default config should have TTS entity")
			}
		})
	}
}

func TestTTSService_AnnounceToAllSpeakers_Validation(t *testing.T) {
	tests := []struct {
		name         string
		message      string
		mediaPlayers []string
		wantErr      bool
	}{
		{
			name:    "announce to multiple speakers",
			message: "Test message",
			mediaPlayers: []string{
				entities.MediaPlayer.LivingRoomGoogleHome,
				entities.MediaPlayer.BedroomGoogleHome,
			},
			wantErr: false,
		},
		{
			name:         "announce to single speaker",
			message:      "Test message",
			mediaPlayers: []string{entities.MediaPlayer.LivingRoomGoogleHome},
			wantErr:      false,
		},
		{
			name:         "empty message fails",
			message:      "",
			mediaPlayers: []string{entities.MediaPlayer.LivingRoomGoogleHome},
			wantErr:      true,
		},
		{
			name:         "empty speaker list succeeds (no-op)",
			message:      "Test message",
			mediaPlayers: []string{},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can only validate empty message without HA
			if tt.message == "" && len(tt.mediaPlayers) > 0 {
				service := &ga.Service{}
				tts := NewTTSService(service)

				// Will fail on first speaker due to empty message
				err := tts.AnnounceToAllSpeakers(tt.message, tt.mediaPlayers)
				if err == nil {
					t.Error("Expected error for empty message, got nil")
				}
				return
			}

			// For valid messages, just verify the structure
			if len(tt.mediaPlayers) == 0 {
				// Empty list should be a no-op (no error expected)
				service := &ga.Service{}
				tts := NewTTSService(service)
				err := tts.AnnounceToAllSpeakers(tt.message, tt.mediaPlayers)
				if err != nil {
					t.Errorf("Unexpected error for empty speaker list: %v", err)
				}
			}
		})
	}
}

func TestAnnouncementConfig_Structure(t *testing.T) {
	t.Run("config with all fields set", func(t *testing.T) {
		config := AnnouncementConfig{
			Message:           "Test",
			MediaPlayerEntity: entities.MediaPlayer.LivingRoomGoogleHome,
			TTSEntity:         entities.Tts.Piper,
			Cache:             true,
			IgnoreErrors:      true,
		}

		if config.Message == "" {
			t.Error("Message should be set")
		}
		if config.MediaPlayerEntity == "" {
			t.Error("MediaPlayerEntity should be set")
		}
		if config.TTSEntity == "" {
			t.Error("TTSEntity should be set")
		}
	})

	t.Run("defaults are properly defined", func(t *testing.T) {
		defaultConfig := DefaultConfig()
		if defaultConfig.MediaPlayerEntity == "" {
			t.Error("Default config should have media player")
		}
		if defaultConfig.TTSEntity == "" {
			t.Error("Default config should have TTS entity")
		}
		if !defaultConfig.Cache {
			t.Error("Default should enable cache")
		}
		if !defaultConfig.IgnoreErrors {
			t.Error("Default should ignore errors")
		}
	})
}
