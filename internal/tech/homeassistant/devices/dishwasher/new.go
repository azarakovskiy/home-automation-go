package dishwasher

import (
	domaindishwasher "home-go/internal/domain/devices/dishwasher"
	domainpricing "home-go/internal/domain/pricing"
	"home-go/internal/tech/homeassistant/component"
	"home-go/internal/tech/homeassistant/notifications"

	ga "saml.dev/gome-assistant"
)

func New(base component.Base, state ga.State, priceService *domainpricing.Service) *domaindishwasher.Dishwasher {
	controller := NewController(base.Service)
	stateManager := NewStateManager(base.Service, state, controller)
	notifier := notifications.NewNotificationService(base.Service)
	return domaindishwasher.New(base, state, priceService, controller, stateManager, notifier)
}
