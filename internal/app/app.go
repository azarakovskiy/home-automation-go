package app

import (
	"fmt"
	"log"
	"os"

	"home-go/internal/config"
	domainpricing "home-go/internal/domain/pricing"
	"home-go/internal/tech/homeassistant/component"
	"home-go/internal/tech/homeassistant/devices/laptop"
	"home-go/internal/tech/homeassistant/devices/dishwasher"
	"home-go/internal/tech/runtime/debug"
	"home-go/internal/tech/runtime/dryrun"

	ga "saml.dev/gome-assistant"
)

// RunFromEnv loads config from the environment and starts the application.
func RunFromEnv() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	return Run(cfg)
}

// Run starts the application with an already loaded config.
func Run(cfg config.Config) error {
	syncRuntimeEnv(cfg)
	dryrun.Init()
	debug.Init()

	app, err := ga.NewApp(ga.NewAppRequest{
		URL:         cfg.HAURL,
		HAAuthToken: cfg.HAAuthToken,
	})
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}
	defer app.Cleanup()

	components := buildComponents(app)
	registerComponents(app, components)
	logStartupInfo(components)

	app.Start()
	return nil
}

func buildComponents(app *ga.App) []component.Component {
	base := component.NewBase(app.GetService())
	priceService := domainpricing.NewService(app.GetService(), app.GetState())

	dishwasherComp := dishwasher.New(base, app.GetState(), priceService)
	laptopChargerComp := laptop.New(base, app.GetState(), priceService)
	// vacuumChargerComp := vacuum.New(base, app.GetState(), priceService)

	return []component.Component{
		priceService,
		dishwasherComp,
		laptopChargerComp,
		// vacuumChargerComp,
	}
}

func registerComponents(app *ga.App, components []component.Component) {
	for _, comp := range components {
		app.RegisterEventListeners(comp.EventListeners()...)
		app.RegisterEntityListeners(comp.EntityListeners()...)
		app.RegisterSchedules(comp.Schedules()...)
		app.RegisterIntervals(comp.Intervals()...)
	}
}

func logStartupInfo(components []component.Component) {
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
