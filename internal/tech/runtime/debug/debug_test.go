package debug

import (
	"os"
	"testing"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{
			name:     "enabled with true",
			envValue: "true",
			want:     true,
		},
		{
			name:     "enabled with TRUE",
			envValue: "TRUE",
			want:     true,
		},
		{
			name:     "disabled with false",
			envValue: "false",
			want:     false,
		},
		{
			name:     "disabled with empty",
			envValue: "",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env var
			os.Setenv("DEBUG", tt.envValue)
			defer os.Unsetenv("DEBUG")

			// Initialize
			Init()

			// Check result
			if IsEnabled() != tt.want {
				t.Errorf("IsEnabled() = %v, want %v", IsEnabled(), tt.want)
			}
		})
	}
}

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
	}{
		{"enabled", true},
		{"disabled", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled = tt.enabled
			if got := IsEnabled(); got != tt.enabled {
				t.Errorf("IsEnabled() = %v, want %v", got, tt.enabled)
			}
		})
	}
}

func TestLog(t *testing.T) {
	tests := []struct {
		name    string
		enabled bool
		format  string
		args    []interface{}
	}{
		{
			name:    "debug enabled logs message",
			enabled: true,
			format:  "Test message: %s",
			args:    []interface{}{"value"},
		},
		{
			name:    "debug disabled skips message",
			enabled: false,
			format:  "Test message: %s",
			args:    []interface{}{"value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled = tt.enabled
			// Just verify it doesn't panic
			Log(tt.format, tt.args...)
		})
	}
}
