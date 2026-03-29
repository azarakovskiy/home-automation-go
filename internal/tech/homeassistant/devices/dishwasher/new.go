package dishwasher

import (
	"context"
	"fmt"

	domaindishwasher "home-go/internal/domain/devices/dishwasher"
	domainpricing "home-go/internal/domain/pricing"
	"home-go/internal/tech/homeassistant/component"
	"home-go/internal/tech/homeassistant/entities"
	"home-go/internal/tech/homeassistant/notifications"

	ga "saml.dev/gome-assistant"
)

func New(base component.Base, state ga.State, priceService *domainpricing.Service, runtime *entities.Runtime) (*domaindishwasher.Dishwasher, error) {
	controller := NewController(base.Service)
	stateManager, err := NewStateManager(runtime, state, controller)
	if err != nil {
		return nil, fmt.Errorf("create state manager: %w", err)
	}

	notifier := notifications.NewNotificationService(base.Service)
	component := domaindishwasher.New(base, state, priceService, controller, stateManager, notifier)

	if err := stateManager.OnScheduledCommand(func(ctx context.Context, on bool) error {
		if on {
			return stateManager.SetScheduledFlag(ctx, component.HasPendingSchedule())
		}
		if !component.HasPendingSchedule() {
			return stateManager.SetScheduledFlag(ctx, false)
		}

		component.CancelPendingScheduleFromDashboard()
		return nil
	}); err != nil {
		return nil, fmt.Errorf("register scheduled switch handler: %w", err)
	}

	return component, nil
}
