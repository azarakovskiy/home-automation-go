package debug

import (
	"log"
	"os"
	"strings"
)

// enabled is a global flag for debug logging
var enabled bool

// Init initializes debug mode based on the DEBUG environment variable
// Must be called at application startup
func Init() {
	enabled = strings.ToLower(os.Getenv("DEBUG")) == "true"
	if enabled {
		log.Printf("🐛 DEBUG MODE ENABLED - Verbose logging active")
	}
}

// IsEnabled returns whether debug mode is active
func IsEnabled() bool {
	return enabled
}

// Log logs a debug message only if debug mode is enabled
// Use this for verbose logging that would clutter normal operation
//
// Example usage:
//
//	debug.Log("Finding best window: now=%s, deadline=%s", now, deadline)
func Log(format string, args ...interface{}) {
	if enabled {
		log.Printf("[DEBUG] "+format, args...)
	}
}
