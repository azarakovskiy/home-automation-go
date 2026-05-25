package priceannouncer

import (
	"fmt"
	"log"
	"time"

	"home-go/internal/domain/pricing"
	"home-go/internal/tech/homeassistant/entities"
	"home-go/internal/tech/homeassistant/notifications"
	"home-go/internal/tech/runtime/debug"

	ga "saml.dev/gome-assistant"
)

// NotificationSender delivers user-facing alerts.
type NotificationSender interface {
	Notify(notifications.Event) error
}

// AnnouncerConfig holds tuneable thresholds.
type AnnouncerConfig struct {
	SpikeMultiplier    float64
	MinExtremeDuration time.Duration
}

// Announcer owns all price announcement logic.
type Announcer struct {
	service            *pricing.Service
	modes              pricing.ModeProvider
	notification       NotificationSender
	cfg                AnnouncerConfig
	now                func() time.Time
	formatter          MessageFormatter
	// lastAlertedRunFrom tracks the start of the most-recently alerted extreme run.
	// In-memory only: at most one duplicate alert per run may fire after a restart — acceptable.
	lastAlertedRunFrom time.Time
}

// New constructs an Announcer.
func New(
	service *pricing.Service,
	modes pricing.ModeProvider,
	notification NotificationSender,
	cfg AnnouncerConfig,
) *Announcer {
	return &Announcer{
		service:      service,
		modes:        modes,
		notification: notification,
		cfg:          cfg,
		now:          time.Now,
		formatter:    naturalLanguageFormatter{},
	}
}

// EventListeners implements component.Component.
func (a *Announcer) EventListeners() []ga.EventListener { return nil }

// EntityListeners registers the reactive price-update trigger.
// On-demand summaries are triggered via the MQTT price summary switch, not here.
func (a *Announcer) EntityListeners() []ga.EntityListener {
	return []ga.EntityListener{
		ga.NewEntityListener().
			EntityIds(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
			Call(a.handlePriceUpdate).
			Build(),
	}
}

// Schedules implements component.Component (unused).
func (a *Announcer) Schedules() []ga.DailySchedule { return nil }

// Intervals implements component.Component (unused).
func (a *Announcer) Intervals() []ga.Interval { return nil }

// HandleOnDemand sends a time-aware price summary immediately.
// Hour < 11 → morning (full day ahead); hour >= 11 → afternoon (remaining window).
func (a *Announcer) HandleOnDemand() {
	if a.isSuppressed() {
		return
	}

	now := a.now()
	idx, err := a.service.CurrentIndex()
	if err != nil {
		log.Printf("Announcer: price index unavailable for on-demand: %v", err)
		return
	}

	midnight := now.Truncate(24 * time.Hour).Add(24 * time.Hour)

	period := PeriodMorning
	if now.Hour() >= 11 {
		period = PeriodAfternoon
	}

	currentSlot, found := idx.SlotAt(now)
	currentLevel := pricing.PriceLevelUnknown
	if found {
		currentLevel = idx.Level(currentSlot)
	}

	summary := idx.Summary(now, midnight)
	ctx := AnnounceContext{
		Period:       period,
		CurrentLevel: currentLevel,
		Summary:      summary,
	}

	msg := a.formatter.Format(ctx)
	if msg == "" {
		debug.Log("Announcer: nothing to announce in on-demand summary")
		return
	}

	if err := a.notification.Notify(notifications.Event{
		Device:  "pricing",
		Type:    "price_day_summary",
		Message: msg,
	}); err != nil {
		log.Printf("Announcer: failed to send on-demand summary: %v", err)
	}
}

func (a *Announcer) handlePriceUpdate(_ *ga.Service, _ ga.State, _ ga.EntityData) {
	if a.isSuppressed() {
		return
	}

	idx, err := a.service.CurrentIndex()
	if err != nil {
		debug.Log("Announcer: index not ready for reactive check: %v", err)
		return
	}

	now := a.now()
	midnight := now.Truncate(24 * time.Hour).Add(24 * time.Hour)

	found, from, till, kind := a.firstExtremeRun(idx, now, midnight)
	if !found {
		return
	}

	// Skip if this is the same run we already alerted on.
	if from.Equal(a.lastAlertedRunFrom) {
		return
	}

	dur := till.Sub(from)
	hours := max(int(dur.Hours()), 1)

	msg := fmt.Sprintf("Heads up: %s for %d consecutive hours starting at %s.",
		kind, hours, from.Format("15:04"))

	if err := a.notification.Notify(notifications.Event{
		Device:  "pricing",
		Type:    "price_extreme_alert",
		Message: msg,
		Data: map[string]any{
			"kind":       kind,
			"from":       from.Format(time.RFC3339),
			"till":       till.Format(time.RFC3339),
			"duration_h": hours,
		},
	}); err != nil {
		log.Printf("Announcer: failed to send reactive alert: %v", err)
		return
	}

	a.lastAlertedRunFrom = from
}

func (a *Announcer) isSuppressed() bool {
	if a.modes == nil {
		return false
	}
	if night, err := a.modes.IsNight(); err != nil {
		log.Printf("Announcer: failed to check night mode: %v", err)
	} else if night {
		return true
	}
	if away, err := a.modes.IsAway(); err != nil {
		log.Printf("Announcer: failed to check away mode: %v", err)
	} else if away {
		return true
	}
	return false
}

func (a *Announcer) firstExtremeRun(idx pricing.PriceIndex, from, deadline time.Time) (bool, time.Time, time.Time, string) {
	var runFrom time.Time
	var runDur time.Duration
	var runKind string
	inRun := false

	for _, s := range idx.Slots() {
		if !s.Till.After(from) || s.From.After(deadline) {
			continue
		}
		if idx.IsExtreme(s, a.cfg.SpikeMultiplier) {
			slotKind := "extreme prices"
			if s.Price < 0 {
				slotKind = "negative prices"
			}
			if !inRun {
				runFrom = s.From
				runKind = slotKind
				inRun = true
				runDur = 0
			}
			runDur += s.Till.Sub(s.From)
		} else {
			if inRun && runDur >= a.cfg.MinExtremeDuration {
				return true, runFrom, runFrom.Add(runDur), runKind
			}
			inRun = false
			runDur = 0
		}
	}
	if inRun && runDur >= a.cfg.MinExtremeDuration {
		return true, runFrom, runFrom.Add(runDur), runKind
	}
	return false, time.Time{}, time.Time{}, ""
}
