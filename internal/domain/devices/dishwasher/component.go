package dishwasher

import (
	"fmt"
	"log"
	"time"

	"home-go/internal/domain/optimizer"
	domainpricing "home-go/internal/domain/pricing"
	domainscheduler "home-go/internal/domain/scheduler"
	"home-go/internal/tech/homeassistant/component"
	"home-go/internal/tech/homeassistant/entities"
	hanotifications "home-go/internal/tech/homeassistant/notifications"

	ga "saml.dev/gome-assistant"
)

// NotificationSender defines the minimal notification surface needed by the dishwasher.
type NotificationSender interface {
	Notify(event hanotifications.Event) error
}

type Controller interface {
	InitializeModeForScheduled(mode string) error
	StartDishwasher() error
}

// Dishwasher handles all dishwasher-related automation
type Dishwasher struct {
	component.Base // Embed Base to get default implementations and common services

	priceService        *domainpricing.Service
	notificationService NotificationSender
	controller          Controller
	optimizer           *optimizer.Optimizer
	scheduler           *domainscheduler.Scheduler
}

type PendingSchedule = domainscheduler.Plan

// New creates a new dishwasher component.
func New(
	base component.Base,
	state ga.State,
	priceService *domainpricing.Service,
	controller Controller,
	scheduler *domainscheduler.Scheduler,
	notificationService NotificationSender,
) *Dishwasher {
	base.State = state

	dishwasher := &Dishwasher{
		Base:                base,
		priceService:        priceService,
		notificationService: notificationService,
		controller:          controller,
		optimizer:           optimizer.NewOptimizer(),
		scheduler:           scheduler,
	}

	if err := dishwasher.scheduler.Restore(time.Now()); err != nil {
		log.Printf("ERROR: Failed to restore schedule: %v", err)
	} else if schedule := dishwasher.scheduler.Pending(); schedule != nil {
		log.Printf("Restored pending schedule: starts at %s",
			schedule.StartTime.Format("15:04"))
	}

	return dishwasher
}

// EventListeners returns event listeners for this component
func (c *Dishwasher) EventListeners() []ga.EventListener {
	// Use typed event handler - no manual parsing needed!
	handler := component.NewTypedEventHandler(
		entities.CustomEvents.ScheduledStart,
		c.handleScheduleRequest,
	)

	return []ga.EventListener{handler.Build()}
}

// EntityListeners returns no direct entity listeners for dishwasher.
// Manual dashboard toggle requests are handled by the HA runtime switch adapter.
func (c *Dishwasher) EntityListeners() []ga.EntityListener {
	return nil
}

// Intervals returns intervals for this component
func (c *Dishwasher) Intervals() []ga.Interval {
	// Check every 5 minutes if it's time to start pending dishwasher
	checkInterval := ga.NewInterval().
		Call(c.checkPendingStart).
		Every("5m").
		Build()

	return []ga.Interval{checkInterval}
}

// handleScheduleRequest processes strongly-typed dishwasher schedule events
// The TypedEventHandler automatically parses the event, so we receive typed data directly
func (c *Dishwasher) handleScheduleRequest(service *ga.Service, state ga.State, request ScheduleRequest) {
	log.Printf("Received schedule request event")

	// Type-safe access to fields - no parsing needed!
	if request.Device != "dishwasher" {
		log.Printf("Event not for dishwasher: %s", request.Device)
		return
	}

	mode := Mode(request.Mode)
	if mode == ModeCancel {
		log.Printf("Received cancel request via scheduled event")
		c.cancelPendingSchedule("cancel event")
		return
	}

	log.Printf("Processing dishwasher schedule: mode=%s, max_delay=%d hours",
		mode, request.MaxDelayHours)

	// Get device profile (strongly typed)
	profile, err := GetProfileForMode(mode)
	if err != nil {
		log.Printf("ERROR: %v", err)
		return
	}

	// Get price slots FIRST - before any device interaction to avoid timing issues
	priceSlots, err := c.priceService.GetPriceSlots()
	if err != nil {
		log.Printf("ERROR: Failed to get prices: %v", err)
		log.Printf("Starting dishwasher immediately (no price data available)")
		if err := c.controller.StartDishwasher(); err != nil {
			log.Printf("ERROR: Failed to start: %v", err)
		}
		c.announceImmediateStart(time.Now(), 0)
		return
	}

	log.Printf("Loaded %d price slots for optimization", len(priceSlots))

	// Use generic optimizer with strongly-typed profile
	// Do ALL expensive calculations BEFORE any device interaction
	result, err := c.optimizer.Optimize(optimizer.OptimizationRequest{
		Profile:       profile, // Implements DeviceProfile interface
		PriceSlots:    priceSlots,
		MaxDelayHours: request.MaxDelayHours,
	})
	if err != nil {
		log.Printf("ERROR: Optimization failed: %v", err)
		log.Printf("Starting dishwasher immediately (optimization failed)")
		if err := c.controller.StartDishwasher(); err != nil {
			log.Printf("ERROR: Failed to start: %v", err)
		}
		c.announceImmediateStart(time.Now(), 0)
		return
	}

	// Check if we're starting immediately due to insufficient data
	if result.Savings == 0 && result.SavingsPercent == 0 {
		log.Printf("Insufficient price data for full cycle optimization")
		log.Printf("Starting immediately with available data")
	}

	log.Printf("Optimization complete:")
	log.Printf("  Start time: %s", result.StartTime.Format("15:04"))
	log.Printf("  Cost: €%.2f (vs €%.2f now)", result.EstimatedCost, result.CurrentCost)
	log.Printf("  Savings: €%.2f (%.1f%%)", result.Savings, result.SavingsPercent)

	// Check if it's night time - if so, accept any savings (no threshold)
	isNight, err := c.IsNightMode()
	if err != nil {
		log.Printf("WARNING: Failed to check daytime mode: %v", err)
		isNight = false // Default to normal threshold logic
	}

	// Decide: start now or delay?
	delayDuration := time.Until(result.StartTime)
	shouldDelay := c.shouldDelayStart(result, request.MaxDelayHours, isNight, delayDuration)

	if !shouldDelay {
		// Get dynamic threshold for logging
		threshold := c.optimizer.CalculateDynamicThreshold(request.MaxDelayHours)
		log.Printf("Starting immediately (savings %.1f%% below threshold of %.1f%% for %dh delay)",
			result.SavingsPercent, threshold, request.MaxDelayHours)

		// Start immediately
		if err := c.controller.StartDishwasher(); err != nil {
			log.Printf("ERROR: Failed to start: %v", err)
			return
		}

		c.announceImmediateStart(time.Now(), result.SavingsPercent)
		return
	}

	// Schedule delayed start
	threshold := c.optimizer.CalculateDynamicThreshold(request.MaxDelayHours)
	log.Printf("Delaying start by %d minutes (savings %.1f%% meets threshold of %.1f%% for %dh delay)",
		int(delayDuration.Minutes()), result.SavingsPercent, threshold, request.MaxDelayHours)

	// Initialize dishwasher NOW (socket on, wait 5s, socket off)
	// This allows user to set the mode, then we wait until optimal time to turn it back on
	if err := c.controller.InitializeModeForScheduled(string(mode)); err != nil {
		log.Printf("ERROR: Failed to initialize: %v", err)
		return
	}

	if err := c.scheduler.Schedule(domainscheduler.Plan{StartTime: result.StartTime}); err != nil {
		log.Printf("ERROR: Failed to save schedule state: %v", err)
		return
	}

	log.Printf("Dishwasher scheduled successfully!")

	// Announce delayed start via TTS
	c.announceDelayedStart(result.StartTime, result.SavingsPercent)
}

// checkPendingStart runs periodically via interval to check if it's time to start
func (c *Dishwasher) checkPendingStart(service *ga.Service, state ga.State) {
	if !c.scheduler.HasPending() {
		return
	}

	if err := c.scheduler.Tick(time.Now()); err != nil {
		log.Printf("ERROR: Failed to start pending dishwasher schedule: %v", err)
	}
}

// announceDelayedStart fires a notification event for a scheduled dishwasher start
func (c *Dishwasher) announceDelayedStart(startTime time.Time, savingsPercent float64) {
	if c.notificationService == nil {
		return
	}

	// Format time in a natural way for speech
	// e.g., "3 PM", "3:30 PM", "noon", "midnight"
	timeStr := formatTimeForSpeech(startTime)

	message := fmt.Sprintf(
		"Dishwasher starts at %s, saving %.0f percent on electricity!",
		timeStr,
		savingsPercent,
	)

	event := hanotifications.Event{
		Device:  "dishwasher",
		Type:    "scheduled",
		Message: message,
		Data: map[string]interface{}{
			"start_time":      startTime.Format("15:04"),
			"start_time_text": timeStr,
			"savings_percent": savingsPercent,
		},
	}

	if err := c.notificationService.Notify(event); err != nil {
		log.Printf("WARNING: Notification event failed: %v", err)
	}
}

// announceImmediateStart fires a notification event when we decide to start right away
func (c *Dishwasher) announceImmediateStart(startTime time.Time, savingsPercent float64) {
	if c.notificationService == nil {
		return
	}

	timeStr := formatTimeForSpeech(startTime)

	message := fmt.Sprintf(
		"Dishwasher starts now, saving %.0f percent on electricity!",
		savingsPercent,
	)

	event := hanotifications.Event{
		Device:  "dishwasher",
		Type:    "started",
		Message: message,
		Data: map[string]interface{}{
			"start_time":      startTime.Format("15:04"),
			"start_time_text": timeStr,
			"savings_percent": savingsPercent,
		},
	}

	if err := c.notificationService.Notify(event); err != nil {
		log.Printf("WARNING: Notification event failed: %v", err)
	}
}

// announceCancellation informs the household that a pending schedule was cancelled.
func (c *Dishwasher) announceCancellation(schedule *PendingSchedule, reason string) {
	if c.notificationService == nil || schedule == nil {
		return
	}

	timeStr := formatTimeForSpeech(schedule.StartTime)
	message := fmt.Sprintf("Dishwasher schedule for %s was cancelled", timeStr)
	if suffix := cancellationReasonToSpeech(reason); suffix != "" {
		message = fmt.Sprintf("%s %s.", message, suffix)
	} else {
		message += "."
	}

	event := hanotifications.Event{
		Device:  "dishwasher",
		Type:    "cancelled",
		Message: message,
		Data: map[string]interface{}{
			"start_time":      schedule.StartTime.Format("15:04"),
			"start_time_text": timeStr,
			"reason":          reason,
		},
	}

	if err := c.notificationService.Notify(event); err != nil {
		log.Printf("WARNING: Notification event failed: %v", err)
	}
}

// shouldDelayStart determines if we should delay the start based on savings and night mode
func (c *Dishwasher) shouldDelayStart(
	result *optimizer.OptimizationResult,
	maxDelayHours int,
	isNight bool,
	delayDuration time.Duration,
) bool {
	// Need at least 5 minutes of delay to be worth it
	if delayDuration < 5*time.Minute {
		return false
	}

	if isNight && result.SavingsPercent > 0 {
		// Night mode: accept any positive savings
		log.Printf("Night mode: accepting any positive savings (%.1f%%)", result.SavingsPercent)
		return true
	}

	// Normal mode: use dynamic threshold
	return c.optimizer.ShouldDelay(result, maxDelayHours)
}

func formatTimeForSpeech(t time.Time) string {
	hour := t.Hour()
	minute := t.Minute()

	if hour == 0 && minute == 0 {
		return "midnight"
	}
	if hour == 12 && minute == 0 {
		return "noon"
	}

	period := "AM"
	displayHour := hour
	if hour >= 12 {
		period = "PM"
		if hour > 12 {
			displayHour = hour - 12
		}
	}
	if displayHour == 0 {
		displayHour = 12
	}

	if minute == 0 {
		return fmt.Sprintf("%d %s", displayHour, period)
	}

	return fmt.Sprintf("%d:%02d %s", displayHour, minute, period)
}

// cancelPendingSchedule clears local + HA state for a pending run
func (c *Dishwasher) cancelPendingSchedule(reason string) {
	schedule := c.scheduler.Pending()
	if schedule == nil {
		log.Printf("Cancellation requested (%s) but no pending dishwasher schedule", reason)
		return
	}

	log.Printf("Cancelling pending dishwasher schedule (%s)", reason)
	if err := c.scheduler.Cancel(); err != nil {
		log.Printf("ERROR: Failed to clear schedule state: %v", err)
		return
	}

	c.announceCancellation(schedule, reason)
}

// HasPendingSchedule reports whether a delayed dishwasher start is currently pending.
func (c *Dishwasher) HasPendingSchedule() bool {
	return c.scheduler.HasPending()
}

// CancelPendingScheduleFromDashboard handles a dashboard-triggered schedule cancellation.
func (c *Dishwasher) CancelPendingScheduleFromDashboard() {
	c.cancelPendingSchedule("dashboard switch turned off")
}

func cancellationReasonToSpeech(reason string) string {
	switch reason {
	case "cancel event":
		return "after a cancel request"
	case "dashboard switch turned off":
		return "manually from Home Assistant"
	default:
		if reason == "" {
			return ""
		}
		return fmt.Sprintf("(%s)", reason)
	}
}
