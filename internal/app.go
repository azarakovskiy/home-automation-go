package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"home-go/internal/config"
	"home-go/internal/domain/pricing"
	domainreminders "home-go/internal/domain/reminders"
	"home-go/internal/tech/homeassistant/component"
	"home-go/internal/tech/homeassistant/devices/dishwasher"
	"home-go/internal/tech/homeassistant/devices/laptop"
	hareminders "home-go/internal/tech/homeassistant/devices/reminders"
	"home-go/internal/tech/homeassistant/entities"
	"home-go/internal/tech/runtime/debug"
	"home-go/internal/tech/runtime/dryrun"
	"home-go/internal/tech/sqlite"

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

	runtimeEntities, err := entities.NewRuntime(entities.RuntimeConfig{
		BrokerURL:       cfg.MQTT.BrokerURL,
		Username:        cfg.MQTT.Username,
		Password:        cfg.MQTT.Password,
		DiscoveryPrefix: cfg.MQTT.DiscoveryPrefix,
		AppPrefix:       cfg.MQTT.AppPrefix,
	})
	if err != nil {
		return fmt.Errorf("failed to create runtime entities: %w", err)
	}
	defer runtimeEntities.Close()

	// V1-49: open SQLite and build the reminders repository.
	db, err := sqlite.Open(cfg.Database)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	remindersRepo := sqlite.NewRemindersRepo(db)

	// V1-50: build the reminders domain manager.
	remindersManager := domainreminders.NewManager(remindersRepo, time.Now)

	components, err := buildComponents(app, runtimeEntities, remindersManager)
	if err != nil {
		return err
	}
	registerComponents(app, components)
	logStartupInfo(components)

	app.Start()
	return nil
}

func buildComponents(app *ga.App, runtimeEntities *entities.Runtime, remindersManager *domainreminders.Manager) ([]component.Component, error) {
	base := component.NewBase(app.GetService())
	priceService := pricing.NewService(app.GetService(), app.GetState())

	dishwasherComp, err := dishwasher.New(base, app.GetState(), priceService, runtimeEntities)
	if err != nil {
		return nil, fmt.Errorf("build dishwasher component: %w", err)
	}
	laptopChargerComp := laptop.New(base, app.GetState(), priceService)
	// vacuumChargerComp := vacuum.New(base, app.GetState(), priceService)

	// V1-51: build and restore the reminders HA component.
	remindersComp := hareminders.New(base, runtimeEntities, remindersManager)
	// V1-52: restore active projections; stale MQTT entities are reconciled inside Restore.
	if err := remindersComp.Restore(context.Background()); err != nil {
		return nil, fmt.Errorf("restore reminders component: %w", err)
	}

	return []component.Component{
		priceService,
		dishwasherComp,
		laptopChargerComp,
		// vacuumChargerComp,
		remindersComp,
	}, nil
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
