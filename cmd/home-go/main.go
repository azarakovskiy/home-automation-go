package main

import (
	"log"
	"os"

	"home-go/component"
	"home-go/debug"
	"home-go/dryrun"
	"home-go/internal/config"
	"home-go/optimization/continuous/laptop"
	"home-go/optimization/scheduled/dishwasher"
	"home-go/pricing"

	ga "saml.dev/gome-assistant"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	syncRuntimeEnv(cfg)

	dryrun.Init()
	debug.Init()

	app, err := ga.NewApp(ga.NewAppRequest{
		URL:         cfg.HAURL,
		HAAuthToken: cfg.HAAuthToken,
	})
	if err != nil {
		log.Fatalf("Failed to create app: %v", err)
	}
	defer app.Cleanup()

	base := component.NewBase(app.GetService())
	priceService := pricing.NewService(app.GetService(), app.GetState())

	dishwasherComp := dishwasher.New(base, app.GetState(), priceService)
	laptopChargerComp := laptop.New(base, app.GetState(), priceService)
	// vacuumChargerComp := vacuum.New(base, app.GetState(), priceService)

	components := []component.Component{
		priceService,
		dishwasherComp,
		laptopChargerComp,
		// vacuumChargerComp,
	}

	for _, comp := range components {
		app.RegisterEventListeners(comp.EventListeners()...)
		app.RegisterEntityListeners(comp.EntityListeners()...)
		app.RegisterSchedules(comp.Schedules()...)
		app.RegisterIntervals(comp.Intervals()...)
	}

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

func syncRuntimeEnv(cfg config.Config) {
	setBoolEnv("DEBUG", cfg.Debug)
	setBoolEnv("DRY_RUN", cfg.DryRun)
}

func setBoolEnv(key string, enabled bool) {
	if enabled {
		_ = os.Setenv(key, "true")
		return
	}

	_ = os.Unsetenv(key)
}
