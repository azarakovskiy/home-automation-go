package app

import (
	"context"
	"fmt"
	"log"
	"time"

	"home-go/internal/config"
	"home-go/internal/domain/devices/priceannouncer"
	"home-go/internal/domain/pricing"
	domainreminders "home-go/internal/domain/reminders"
	"home-go/internal/tech/homeassistant/component"
	"home-go/internal/tech/homeassistant/devices/dishwasher"
	svchealth "home-go/internal/tech/homeassistant/devices/health"
	"home-go/internal/tech/homeassistant/devices/laptop"
	hareminders "home-go/internal/tech/homeassistant/devices/reminders"
	"home-go/internal/tech/homeassistant/entities"
	"home-go/internal/tech/homeassistant/notifications"
	apphttp "home-go/internal/tech/http"
	healthhttp "home-go/internal/tech/http/health"
	noisehttp "home-go/internal/tech/http/noise"
	"home-go/internal/tech/postgres"
	"home-go/internal/tech/runtime/debug"
	"home-go/internal/tech/runtime/dryrun"

	ga "saml.dev/gome-assistant"
)

type haModeProvider struct {
	state ga.State
}

func (p *haModeProvider) IsNight() (bool, error) {
	s, err := p.state.Get(entities.InputSelect.DaytimeMode)
	if err != nil {
		return false, fmt.Errorf("get daytime mode: %w", err)
	}
	return s.State == "Night", nil
}

func (p *haModeProvider) IsAway() (bool, error) {
	s, err := p.state.Get(entities.InputSelect.HouseMode)
	if err != nil {
		return false, fmt.Errorf("get house mode: %w", err)
	}
	return s.State == "Away" || s.State == "Travel", nil
}

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
	dryrun.Init(cfg.DryRun)
	debug.Init(cfg.Debug)
	startTime := time.Now()

	app, err := ga.NewApp(ga.NewAppRequest{
		URL:         cfg.HAURL,
		HAAuthToken: cfg.HAAuthToken,
	})
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}
	defer app.Cleanup()

	runtimeEntities, err := entities.NewRuntime(entities.RuntimeConfig{
		BrokerURL:           cfg.MQTT.BrokerURL,
		Username:            cfg.MQTT.Username,
		Password:            cfg.MQTT.Password,
		DiscoveryPrefix:     cfg.MQTT.DiscoveryPrefix,
		AppPrefix:           cfg.MQTT.AppPrefix,
		AppName:             cfg.MQTT.AppName,
		DeviceNameSeparator: cfg.MQTT.DeviceNameSeparator,
	})
	if err != nil {
		return fmt.Errorf("failed to create runtime entities: %w", err)
	}
	defer runtimeEntities.Close()

	db, err := postgres.Open(cfg.Database)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	remindersRepo := postgres.NewRemindersRepo(db)

	notifSvc := notifications.NewNotificationService(app.GetService())
	remindersManager := domainreminders.NewManager(remindersRepo, notifications.NewReminderNotifier(notifSvc), time.Now)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	healthHTTPHandler := healthhttp.New(startTime)
	noiseHTTPHandler := &noisehttp.Handler{}
	srv := apphttp.NewServer(cfg.HTTP.Host, cfg.HTTP.Port, noiseHTTPHandler.ServeNoise, healthHTTPHandler.ServeHealth)
	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Printf("ERROR: HTTP server: %v", err)
		}
	}()

	components, err := buildComponents(ctx, app, runtimeEntities, remindersManager, startTime, cfg.MQTT.AppPrefix)
	if err != nil {
		return err
	}
	registerComponents(app, components)
	logStartupInfo(components)

	app.Start()
	return nil
}

func buildComponents(ctx context.Context, app *ga.App, runtimeEntities *entities.Runtime, remindersManager *domainreminders.Manager, startTime time.Time, mqttPrefix string) ([]component.Component, error) {
	base := component.NewBase(app.GetService())
	priceService := pricing.NewService(app.GetState())

	notifier := notifications.NewNotificationService(app.GetService())
	modeProvider := &haModeProvider{state: app.GetState()}
	announcerComp := priceannouncer.New(priceService, modeProvider, notifier, priceannouncer.AnnouncerConfig{
		SpikeMultiplier:    3.0,
		MinExtremeDuration: time.Hour,
	})

	// Reactive trigger: fire HandlePriceUpdate on every price-sensor change.
	// Entity ID lives here (app layer) rather than in the domain.
	app.RegisterEntityListeners(
		ga.NewEntityListener().
			EntityIds(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
			Call(announcerComp.HandlePriceUpdate).
			Build(),
	)

	// On-demand trigger: an MQTT switch that sends the day summary when turned ON.
	priceSummarySwitch, err := runtimeEntities.Switch(ctx, entities.SwitchSpec{
		CommonSpec: entities.CommonSpec{
			Key:  "price_summary_trigger",
			Name: "Price Summary",
			Icon: "mdi:currency-eur",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("declare price summary switch: %w", err)
	}
	if err := priceSummarySwitch.OnCommand(func(_ context.Context, on bool) error {
		if on {
			announcerComp.HandleOnDemand()
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("register price summary command handler: %w", err)
	}

	dishwasherComp, err := dishwasher.New(base, app.GetState(), priceService, runtimeEntities)
	if err != nil {
		return nil, fmt.Errorf("build dishwasher component: %w", err)
	}
	laptopChargerComp := laptop.New(base, app.GetState(), priceService)
	// vacuumChargerComp := vacuum.New(base, app.GetState(), priceService)

	remindersHandler := hareminders.New(base, runtimeEntities, remindersManager, hareminders.AdaptDeviceRuntime(runtimeEntities.ForDevice("Reminders")), mqttPrefix)
	if err := remindersHandler.Start(ctx); err != nil {
		return nil, fmt.Errorf("start reminders handler: %w", err)
	}

	healthComp, err := svchealth.New(ctx, base, runtimeEntities, startTime)
	if err != nil {
		return nil, fmt.Errorf("build health component: %w", err)
	}

	return []component.Component{
		priceService,
		announcerComp,
		dishwasherComp,
		laptopChargerComp,
		// vacuumChargerComp,
		remindersHandler,
		healthComp,
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
