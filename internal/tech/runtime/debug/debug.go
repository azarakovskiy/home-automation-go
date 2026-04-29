package debug

import (
	"log"
	"os"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// enabled is a global flag for debug logging
var enabled bool

// Init initializes debug mode.
// Must be called at application startup.
func Init(dbg bool) {
	enabled = dbg
	if enabled {
		log.Printf("🐛 DEBUG MODE ENABLED - Verbose logging active")
		mqtt.ERROR = log.New(os.Stdout, "[ERROR] ", 0)
		mqtt.CRITICAL = log.New(os.Stdout, "[CRIT] ", 0)
		mqtt.WARN = log.New(os.Stdout, "[WARN]  ", 0)
		mqtt.DEBUG = log.New(os.Stdout, "[DEBUG] ", 0)
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
