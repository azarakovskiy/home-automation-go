package dishwasher

import (
	"context"
	"fmt"
	"log"

	domaindishwasher "home-go/internal/domain/devices/dishwasher"
	domainpricing "home-go/internal/domain/pricing"
	domainscheduler "home-go/internal/domain/scheduler"
	"home-go/internal/tech/homeassistant/component"
	"home-go/internal/tech/homeassistant/entities"
	"home-go/internal/tech/homeassistant/notifications"
	haschedule "home-go/internal/tech/homeassistant/schedule"

	ga "saml.dev/gome-assistant"
)

const (
	dishwasherScheduledKey      = "kitchen_dishwasher_is_scheduled"
	dishwasherScheduledStartKey = "kitchen_dishwasher_scheduled_start"
)

func New(base component.Base, state ga.State, priceService *domainpricing.Service, runtime *entities.Runtime) (*domaindishwasher.Dishwasher, error) {
	controller := NewController(base.Service)
	scheduleStore, err := haschedule.NewStore(runtime, state, haschedule.Config{
		Scheduled: entities.SwitchSpec{
			CommonSpec: entities.CommonSpec{
				Key:          dishwasherScheduledKey,
				Name:         "Kitchen Dishwasher: Is Scheduled",
				EntityIDHint: "switch.kitchen_dishwasher_is_scheduled",
				Icon:         "mdi:dishwasher-alert",
			},
		},
		StartTime: entities.TextSensorSpec{
			CommonSpec: entities.CommonSpec{
				Key:          dishwasherScheduledStartKey,
				Name:         "Kitchen Dishwasher: Scheduled Start Time",
				EntityIDHint: "sensor.kitchen_dishwasher_scheduled_start",
				Icon:         "mdi:clock-start",
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create schedule store: %w", err)
	}

	notifier := notifications.NewNotificationService(base.Service)
	scheduler := domainscheduler.New(scheduleStore, schedulerRunner{
		state:      state,
		controller: controller,
	})
	component := domaindishwasher.New(base, state, priceService, controller, scheduler, notifier)

	if err := scheduleStore.OnCommand(func(ctx context.Context, on bool) error {
		if on {
			return scheduleStore.SetScheduledFlag(ctx, component.HasPendingSchedule())
		}
		if !component.HasPendingSchedule() {
			return scheduleStore.SetScheduledFlag(ctx, false)
		}

		component.CancelPendingScheduleFromDashboard()
		return nil
	}); err != nil {
		return nil, fmt.Errorf("register scheduled switch handler: %w", err)
	}

	return component, nil
}

type schedulerRunner struct {
	state      ga.State
	controller scheduleStarter
}

type scheduleStarter interface {
	StartDishwasher() error
}

func (r schedulerRunner) StartNow() error {
	return r.controller.StartDishwasher()
}

func (r schedulerRunner) HandleExpiredSchedule() error {
	log.Printf("Restored dishwasher schedule has already passed, checking whether it still needs to start")

	socketState, err := r.state.Get(entities.Switch.KitchenDishwasherSocket)
	if err != nil {
		return fmt.Errorf("check dishwasher socket state: %w", err)
	}
	if socketState.State == "on" {
		log.Printf("Dishwasher socket is already on, leaving expired schedule as already running")
		return nil
	}

	log.Printf("Dishwasher socket is off for expired schedule, starting now")
	return r.controller.StartDishwasher()
}
