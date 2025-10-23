package dishwasher

import (
	"testing"

	ga "saml.dev/gome-assistant"
)

// Integration-style tests that verify the controller's behavior
func TestController_NewController(t *testing.T) {
	t.Run("controller can be created", func(t *testing.T) {
		service := &ga.Service{}
		controller := NewController(service)

		if controller == nil {
			t.Fatal("NewController returned nil")
		}

		if controller.service != service {
			t.Error("Controller service not set correctly")
		}
	})
}

// Note: Full unit tests for InitializeModeForScheduled and StartDishwasher
// would require mocking the ga.Service.Switch methods.
// These are better tested as integration tests with a real Home Assistant instance
// or by creating wrapper interfaces for the Service.
