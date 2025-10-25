package notifications

import (
	"strings"
	"testing"
)

func TestGetTerryMessage(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		wantKey  string
		contains []string // Check if message contains these strings (for Terry's style)
	}{
		{
			name:     "Dishwasher now",
			key:      "dishwasher_now",
			wantKey:  "dishwasher_now",
			contains: []string{"Terry"}, // Just check for Terry, not specific words since we have 10 variations
		},
		{
			name:     "Dishwasher later",
			key:      "dishwasher_later",
			wantKey:  "dishwasher_later",
			contains: []string{"{{time}}", "{{savings}}"},
		},
		{
			name:    "Unknown key returns key itself",
			key:     "unknown_notification",
			wantKey: "unknown_notification",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetTerryMessage(tt.key)

			// If no translation exists, should return the key
			if len(tt.contains) == 0 {
				if got != tt.key {
					t.Errorf("GetTerryMessage(%q) = %q, want %q (fallback to key)", tt.key, got, tt.key)
				}
				return
			}

			// Check if message contains expected Terry-style elements
			for _, substr := range tt.contains {
				if !strings.Contains(got, substr) {
					t.Errorf("GetTerryMessage(%q) = %q, should contain %q", tt.key, got, substr)
				}
			}

			// Ensure we got a non-empty message
			if got == "" {
				t.Errorf("GetTerryMessage(%q) returned empty string", tt.key)
			}
		})
	}
}

func TestGetTerryMessageOrDefault(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		defaultMsg string
		wantMsg    string
		checkStyle bool // If true, verify it's a Terry message (not default)
	}{
		{
			name:       "Existing key returns Terry message",
			key:        "dishwasher_now",
			defaultMsg: "Start dishwasher",
			checkStyle: true, // Should return Terry style, not default
		},
		{
			name:       "Unknown key returns default",
			key:        "unknown_key",
			defaultMsg: "Default message",
			wantMsg:    "Default message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetTerryMessageOrDefault(tt.key, tt.defaultMsg)

			if tt.checkStyle {
				// Should get Terry-style message, not default
				if got == tt.defaultMsg {
					t.Errorf("GetTerryMessageOrDefault(%q) returned default instead of Terry message", tt.key)
				}
				if !strings.Contains(got, "Terry") && !strings.Contains(got, "TERRY") {
					t.Errorf("GetTerryMessageOrDefault(%q) = %q, should contain 'Terry' (signature style)", tt.key, got)
				}
			} else {
				// Should get default message
				if got != tt.wantMsg {
					t.Errorf("GetTerryMessageOrDefault(%q) = %q, want %q", tt.key, got, tt.wantMsg)
				}
			}
		})
	}
}

func TestTerryTranslations_Coverage(t *testing.T) {
	// Just ensure translations exist and aren't empty
	for key, variations := range TerryTranslations {
		for idx, msg := range variations {
			t.Run(key+"_"+string(rune(idx+'0')), func(t *testing.T) {
				// Message should not be empty
				if msg == "" {
					t.Errorf("Translation %q[%d] is empty", key, idx)
				}

				// Message should be reasonably short for notifications (< 150 chars)
				if len(msg) > 150 {
					t.Errorf("Translation %q[%d] is too long (%d chars), should be < 150 for notifications", key, idx, len(msg))
				}
			})
		}
	}
}

func TestGetTerryMessageWithData(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		data     map[string]string
		contains string
	}{
		{
			name:     "Replace savings and time placeholders",
			key:      "dishwasher_later",
			data:     map[string]string{"savings": "15", "time": "3 PM"},
			contains: "15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetTerryMessageWithData(tt.key, tt.data)

			if !strings.Contains(got, tt.contains) {
				t.Errorf("GetTerryMessageWithData(%q, %v) = %q, should contain %q", tt.key, tt.data, got, tt.contains)
			}

			// Should not contain unreplaced placeholders
			if strings.Contains(got, "{{") || strings.Contains(got, "}}") {
				t.Errorf("GetTerryMessageWithData(%q, %v) = %q, contains unreplaced placeholders", tt.key, tt.data, got)
			}
		})
	}
}

func TestTerryTranslations_Count(t *testing.T) {
	// Ensure we have dishwasher translations
	count := len(TerryTranslations)
	if count < 2 {
		t.Errorf("Expected at least 2 Terry translations (now/later), got %d", count)
	}

	// Each should have 10 variations
	for key, variations := range TerryTranslations {
		if len(variations) != 10 {
			t.Errorf("Expected 10 variations for %q, got %d", key, len(variations))
		}
	}
}
