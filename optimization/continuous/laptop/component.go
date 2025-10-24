package laptop

import (
	"log"
	"time"

	"home-go/component"
	"home-go/dryrun"
	"home-go/entities"
	"home-go/optimization/continuous"
	"home-go/optimization/optimizer"
	"home-go/pricing"

	ga "saml.dev/gome-assistant"
)

// LaptopCharger manages laptop charging optimization
type LaptopCharger struct {
	component.Base

	priceService *pricing.Service
	optimizer    *optimizer.Optimizer
	profile      continuous.ChargingProfile
}

// New creates a new laptop charger component
func New(base component.Base, state ga.State, priceService *pricing.Service) *LaptopCharger {
	base.State = state

	return &LaptopCharger{
		Base:         base,
		priceService: priceService,
		optimizer:    optimizer.NewOptimizer(),
		profile:      continuous.LaptopProfile,
	}
}

// Intervals returns 15-minute interval for optimization checks
func (c *LaptopCharger) Intervals() []ga.Interval {
	return []ga.Interval{
		ga.NewInterval().
			Call(c.optimizeCharging).
			Every("15m").
			Build(),
	}
}

// optimizeCharging runs every 15 minutes to decide if laptop should charge now
func (c *LaptopCharger) optimizeCharging(service *ga.Service, state ga.State) {
	log.Printf("Running laptop charger optimization")

	// Safety check: turn off if away for >2 hours
	awayTooLong, err := c.IsAwayForDuration(2 * time.Hour)
	if err != nil {
		log.Printf("WARNING: Failed to check house mode: %v", err)
	}
	if awayTooLong {
		log.Printf("House away >2h, turning off laptop charger for safety")
		if err := c.turnOff(); err != nil {
			log.Printf("ERROR: Failed to turn off: %v", err)
		}
		return
	}

	// Check if auto-optimization is enabled
	autoState, err := state.Get(entities.InputBoolean.OfficeLaptopChargeOptimizationAuto)
	if err != nil {
		log.Printf("ERROR: Failed to get auto-optimization state: %v", err)
		return
	}

	if autoState.State != "on" {
		log.Printf("Laptop charge optimization disabled, skipping")
		return
	}

	// Get price slots
	priceSlots, err := c.priceService.GetPriceSlots()
	if err != nil {
		log.Printf("ERROR: Failed to get prices: %v", err)
		return
	}

	// Optimize using predefined profile
	result, err := c.optimizer.OptimizeCheapestHours(c.profile.ToOptimizerRequest(), priceSlots)
	if err != nil {
		log.Printf("ERROR: Optimization failed: %v", err)
		return
	}

	// Apply decision
	if result.ChargeNow {
		log.Printf("Laptop: Charging now (current slot is cheap, savings: %.1f%%, will charge for %s)",
			result.SavingsPercent, result.TotalDuration)
		if err := c.turnOn(); err != nil {
			log.Printf("ERROR: Failed to turn on: %v", err)
		}
	} else {
		log.Printf("Laptop: Not charging now (waiting for cheaper slots)")
		if err := c.turnOff(); err != nil {
			log.Printf("ERROR: Failed to turn off: %v", err)
		}
	}
}

func (c *LaptopCharger) turnOn() error {
	return dryrun.Call("Switch.TurnOn", entities.Switch.OfficeLaptopSocket, func() error {
		return c.Service.Switch.TurnOn(entities.Switch.OfficeLaptopSocket)
	})
}

func (c *LaptopCharger) turnOff() error {
	return dryrun.Call("Switch.TurnOff", entities.Switch.OfficeLaptopSocket, func() error {
		return c.Service.Switch.TurnOff(entities.Switch.OfficeLaptopSocket)
	})
}
