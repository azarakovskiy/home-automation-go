package reminders

import (
	domainreminders "home-go/internal/domain/reminders"
	"home-go/internal/tech/homeassistant/component"
	"home-go/internal/tech/homeassistant/entities"
)

// New constructs a reminders Component wiring the domain manager to Home
// Assistant custom events and MQTT runtime projections.
//
// Call Restore on the returned Component during app startup to rebuild
// projections for reminders that were active before the last restart.
func New(base component.Base, runtime *entities.Runtime, manager *domainreminders.Manager) *Component {
	return &Component{
		Base:    base,
		runtime: &runtimeProjector{rt: runtime},
		manager: manager,
	}
}
