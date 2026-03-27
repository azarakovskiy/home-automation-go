package vacuum

import (
	domainvacuum "home-go/internal/domain/devices/vacuum"
	domainpricing "home-go/internal/domain/pricing"
	"home-go/internal/tech/homeassistant/component"
	"home-go/internal/tech/homeassistant/entities"
	"home-go/internal/tech/runtime/dryrun"

	ga "saml.dev/gome-assistant"
)

type Controller struct {
	service *ga.Service
}

func NewController(service *ga.Service) *Controller {
	return &Controller{service: service}
}

func (c *Controller) TurnOn() error {
	return dryrun.Call("Switch.TurnOn", entities.Switch.LivingRoomVacuumSocket, func() error {
		return c.service.Switch.TurnOn(entities.Switch.LivingRoomVacuumSocket)
	})
}

func (c *Controller) TurnOff() error {
	return dryrun.Call("Switch.TurnOff", entities.Switch.LivingRoomVacuumSocket, func() error {
		return c.service.Switch.TurnOff(entities.Switch.LivingRoomVacuumSocket)
	})
}

func New(base component.Base, state ga.State, priceService *domainpricing.Service) *domainvacuum.VacuumCharger {
	return domainvacuum.New(base, state, priceService, NewController(base.Service), Profile)
}
