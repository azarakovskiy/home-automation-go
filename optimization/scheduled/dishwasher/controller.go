package dishwasher

import (
	"fmt"
	"log"
	"time"

	"home-go/dryrun"
	"home-go/entities"

	ga "saml.dev/gome-assistant"
)

// Controller handles dishwasher socket control
type Controller struct {
	service *ga.Service
}

func NewController(service *ga.Service) *Controller {
	return &Controller{
		service: service,
	}
}

// InitializeModeForScheduled turns on socket briefly to allow mode setting
// This is the workaround for non-smart dishwasher: turn on socket,
// wait 5 seconds for user to set mode, then turn off to prevent immediate start
func (c *Controller) InitializeModeForScheduled(mode string) error {
	log.Printf("Initializing dishwasher for mode: %s", mode)

	// Turn on the dishwasher socket
	if err := c.turnOnSocket(); err != nil {
		return fmt.Errorf("failed to turn on socket: %w", err)
	}

	log.Printf("Socket ON - User has 5 seconds to set mode to: %s", mode)

	// Wait 5 seconds for user to set the mode on the dishwasher
	time.Sleep(5 * time.Second) // todo: make this configurable

	// Turn off socket to prevent immediate start
	if err := c.turnOffSocket(); err != nil {
		return fmt.Errorf("failed to turn off socket: %w", err)
	}

	log.Printf("Socket OFF - Dishwasher ready for delayed start")
	return nil
}

// StartDishwasher turns on the socket to begin the cycle
func (c *Controller) StartDishwasher() error {
	log.Printf("Starting dishwasher cycle NOW")

	if err := c.turnOnSocket(); err != nil {
		return fmt.Errorf("failed to start dishwasher: %w", err)
	}

	return nil
}

// turnOnSocket turns on the dishwasher socket
func (c *Controller) turnOnSocket() error {
	// Using the kitchen dishwasher socket entity from entities package
	return dryrun.Call("Switch.TurnOn", entities.Switch.KitchenDishwasherSocket, func() error {
		return c.service.Switch.TurnOn(entities.Switch.KitchenDishwasherSocket)
	})
}

// turnOffSocket turns off the dishwasher socket
func (c *Controller) turnOffSocket() error {
	return dryrun.Call("Switch.TurnOff", entities.Switch.KitchenDishwasherSocket, func() error {
		return c.service.Switch.TurnOff(entities.Switch.KitchenDishwasherSocket)
	})
}
