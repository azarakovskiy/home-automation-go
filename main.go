package main

import (
	"log"
	"os"

	"home-go/charger/laptop"
	"home-go/charger/vacuum"
	"home-go/component"
	"home-go/debug"
	"home-go/dryrun"
	"home-go/pricing"
	"home-go/scheduler/dishwasher"

	ga "saml.dev/gome-assistant"
)

func main() {
	// Get Home Assistant configuration from environment
	haURL := os.Getenv("HA_URL")
	if haURL == "" {
		log.Fatalf("HA_URL is not set")
	}
	authToken := os.Getenv("HA_AUTH_TOKEN")
	if authToken == "" {
		log.Fatalf("HA_AUTH_TOKEN is not set")
	}

	// Initialize dry-run and debug modes
	dryrun.Init()
	debug.Init()

	app, err := ga.NewApp(ga.NewAppRequest{
		URL:         haURL,
		HAAuthToken: authToken,
	})
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}
	defer app.Cleanup()

	// Initialize shared services and base component
	base := component.NewBase(app.GetService())
	priceService := pricing.NewService(app.GetState())

	// Initialize components - pass shared base and state
	dishwasherComp := dishwasher.New(base, app.GetState(), priceService)
	laptopChargerComp := laptop.New(base, app.GetState(), priceService)
	vacuumChargerComp := vacuum.New(base, app.GetState(), priceService)

	// Collect all components
	components := []component.Component{
		dishwasherComp,
		laptopChargerComp,
		vacuumChargerComp,
	}

	// Register all listeners from components
	for _, comp := range components {
		app.RegisterEventListeners(comp.EventListeners()...)
		app.RegisterEntityListeners(comp.EntityListeners()...)
		app.RegisterSchedules(comp.Schedules()...)
		app.RegisterIntervals(comp.Intervals()...)
	}

	// Log startup info
	if dryrun.IsEnabled() {
		log.Printf("🔧 DRY-RUN MODE ENABLED")
	}

	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("🏠 Starting Home Automation with %d components:", len(components))
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for _, comp := range components {
		eventCount := len(comp.EventListeners())
		entityCount := len(comp.EntityListeners())
		scheduleCount := len(comp.Schedules())
		intervalCount := len(comp.Intervals())

		log.Printf("  📦 %T", comp)
		if eventCount > 0 {
			log.Printf("     ⚡ %d event listener(s)", eventCount)
		}
		if entityCount > 0 {
			log.Printf("     🔔 %d entity listener(s)", entityCount)
		}
		if scheduleCount > 0 {
			log.Printf("     ⏰ %d daily schedule(s)", scheduleCount)
		}
		if intervalCount > 0 {
			log.Printf("     🔄 %d interval(s)", intervalCount)
		}
	}

	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	app.Start()
}

// blinkLights is an example entity listener (disabled)
// func blinkLights(service *ga.Service, state ga.State, sensor ga.EntityData) {
// 	light := entities.Light.LivingRoomBlackLamp
// 	switch sensor.ToState {
// 	case "on":
// 		if err := service.HomeAssistant.TurnOn(light); err != nil {
// 			log.Printf("Failed to turn on light: %v", err)
// 		}
// 	case "off":
// 		if err := service.HomeAssistant.TurnOff(light); err != nil {
// 			log.Printf("Failed to turn off light: %v", err)
// 		}
// 	}
// }
