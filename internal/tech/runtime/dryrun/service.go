package dryrun

import (
	"log"
	"os"
	"strings"
)

// enabled is a global flag for dry-run mode
var enabled bool

// Init initializes dry-run mode based on the DRY_RUN environment variable
// Must be called at application startup before any services are used
func Init() {
	enabled = strings.ToLower(os.Getenv("DRY_RUN")) == "true"
	if enabled {
		log.Printf("🔧 DRY-RUN MODE ENABLED - No actual device changes will be made")
	}
}

// IsEnabled returns whether dry-run mode is active
func IsEnabled() bool {
	return enabled
}

// Call wraps a service call with dry-run logging
// If dry-run is enabled, logs the action and skips execution
// Otherwise, executes the provided function
//
// Example usage:
//
//	dryrun.Call("Switch.TurnOn", "switch.kitchen_dishwasher", func() error {
//	    return service.Switch.TurnOn(entityID)
//	})
func Call(action string, entityID string, fn func() error) error {
	if enabled {
		log.Printf("[DRY-RUN] %s: %s", action, entityID)
		return nil
	}
	return fn()
}

// CallWithData wraps a service call that includes additional data
// If dry-run is enabled, logs the action with data and skips execution
//
// Example usage:
//
//	dryrun.CallWithData("InputNumber.Set", entityID, value, func() error {
//	    return service.InputNumber.Set(entityID, value)
//	})
func CallWithData(action string, entityID string, data interface{}, fn func() error) error {
	if enabled {
		log.Printf("[DRY-RUN] %s: %s = %v", action, entityID, data)
		return nil
	}
	return fn()
}
