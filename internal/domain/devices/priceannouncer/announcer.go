package priceannouncer

import (
	"context"
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

// AnnouncerStateStore persists deduplication state across restarts.
type AnnouncerStateStore interface {
	LastAnnouncedDate(ctx context.Context) (time.Time, error)
	SetLastAnnouncedDate(ctx context.Context, t time.Time) error
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
	db                 AnnouncerStateStore
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
	db AnnouncerStateStore,
	cfg AnnouncerConfig,
) *Announcer {
	return &Announcer{
		service:      service,
		modes:        modes,
		notification: notification,
		db:           db,
		cfg:          cfg,
		now:          time.Now,
		formatter:    naturalLanguageFormatter{},
	}
}

// EventListeners implements component.Component.
func (a *Announcer) EventListeners() []ga.EventListener { return nil }

// EntityListeners registers the reactive price-update trigger.
// The morning trigger is registered externally by the caller via HandleMorning.
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

// HandleMorning sends the daily morning summary if conditions allow.
// Register this against any entity whose state change signals morning (e.g. daytime mode).
func (a *Announcer) HandleMorning(_ *ga.Service, _ ga.State, _ ga.EntityData) {
	if a.isSuppressed() {
		return
	}

	ctx := context.Background()
	last, err := a.db.LastAnnouncedDate(ctx)
	if err != nil {
		log.Printf("Announcer: failed to read last announced date: %v", err)
		return
	}

	today := a.now().Truncate(24 * time.Hour)
	if !last.Before(today) {
		debug.Log("Announcer: morning summary already sent today, skipping")
		return
	}

	if err := a.db.SetLastAnnouncedDate(ctx, a.now()); err != nil {
		log.Printf("Announcer: failed to persist announced date: %v", err)
		return
	}

	a.sendDaySummary()
}

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

func (a *Announcer) sendDaySummary() {
	idx, err := a.service.CurrentIndex()
	if err != nil {
		log.Printf("Announcer: price index unavailable for day summary: %v", err)
		return
	}

	now := a.now()
	midnight := now.Truncate(24 * time.Hour).Add(24 * time.Hour)
	summary := idx.Summary(now, midnight)

	msg := formatDaySummary(summary)
	if msg == "" {
		debug.Log("Announcer: nothing to announce in day summary")
		return
	}

	if err := a.notification.Notify(notifications.Event{
		Device:  "pricing",
		Type:    "price_day_summary",
		Message: msg,
	}); err != nil {
		log.Printf("Announcer: failed to send day summary: %v", err)
	}
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

func formatDaySummary(summary pricing.IndexSummary) string {
	if len(summary.CheapWindows) == 0 && len(summary.ExpensiveWindows) == 0 && len(summary.NegativeWindows) == 0 {
		return ""
	}

	msg := fmt.Sprintf("Electricity prices today (median %.0f ct/kWh).", summary.MedianPrice*100)

	if len(summary.NegativeWindows) > 0 {
		w := summary.NegativeWindows[0]
		msg += fmt.Sprintf(" Negative prices %s–%s.", w.From.Format("15:04"), w.Till.Format("15:04"))
	}

	if len(summary.CheapWindows) > 0 {
		w := summary.CheapWindows[0]
		msg += fmt.Sprintf(" Cheap window %s–%s.", w.From.Format("15:04"), w.Till.Format("15:04"))
	}

	if len(summary.ExpensiveWindows) > 0 {
		w := summary.ExpensiveWindows[0]
		msg += fmt.Sprintf(" Expensive %s–%s.", w.From.Format("15:04"), w.Till.Format("15:04"))
	}

	return msg
}
