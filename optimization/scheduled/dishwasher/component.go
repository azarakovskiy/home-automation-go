package dishwasher

import (
	"fmt"
	"log"
	"time"

	"home-go/entities"
	"home-go/internal/domain/optimizer"
	"home-go/internal/tech/homeassistant/component"
	"home-go/internal/tech/homeassistant/notifications"
	"home-go/optimization/scheduled"
	"home-go/pricing"

	ga "saml.dev/gome-assistant"
)

// NotificationSender defines the minimal notification surface needed by the dishwasher.
type NotificationSender interface {
	Notify(notifications.NotificationEvent) error
}

// Dishwasher handles all dishwasher-related automation
type Dishwasher struct {
	component.Base // Embed Base to get default implementations and common services

	priceService        *pricing.Service
	notificationService NotificationSender
	controller          *Controller
	optimizer           *optimizer.Optimizer
	stateManager        ScheduleStateStore

	pendingSchedule *PendingSchedule
}

// PendingSchedule tracks a scheduled dishwasher cycle
type PendingSchedule struct {
	Mode      Mode
	StartTime time.Time
	Result    *optimizer.OptimizationResult
}

// ScheduleStateStore captures the persistence surface used by the component.
// It allows us to inject fakes in tests without touching Home Assistant services.
//
//go:generate mockgen -destination=../../../internal/mocks/optimization/scheduled/dishwasher/scheduled_state_store.go -package=dishwasher home-go/optimization/scheduled/dishwasher ScheduleStateStore
type ScheduleStateStore interface {
	SaveSchedule(*PendingSchedule) error
	RestoreSchedule() (*PendingSchedule, error)
	ClearSchedule() error
	IsScheduleCancelled() (bool, error)
}

// New creates a new dishwasher component
func New(base component.Base, state ga.State, priceService *pricing.Service) *Dishwasher {
	// Set the State field in base for IsNightMode() to work
	base.State = state

	controller := NewController(base.Service)

	dishwasher := &Dishwasher{
		Base:                base,
		priceService:        priceService,
		notificationService: notifications.NewNotificationService(base.Service),
		controller:          controller,
		optimizer:           optimizer.NewOptimizer(),
		stateManager:        NewStateManager(base.Service, state, controller),
	}

	// Attempt to restore schedule from HASS on startup
	if schedule, err := dishwasher.stateManager.RestoreSchedule(); err != nil {
		log.Printf("ERROR: Failed to restore schedule: %v", err)
	} else if schedule != nil {
		dishwasher.pendingSchedule = schedule
		log.Printf("Restored pending schedule: mode=%s, starts at %s",
			schedule.Mode, schedule.StartTime.Format("15:04"))
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

// EntityListeners listens for manual cancellations via the scheduled flag helper
func (c *Dishwasher) EntityListeners() []ga.EntityListener {
	listener := ga.NewEntityListener().
		EntityIds(entities.InputBoolean.KitchenDishwasherIsScheduled).
		Call(c.handleScheduleFlagChange).
		Build()

	return []ga.EntityListener{listener}
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
func (c *Dishwasher) handleScheduleRequest(service *ga.Service, state ga.State, request scheduled.ScheduleRequest) {
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

	// Store pending schedule in memory
	c.pendingSchedule = &PendingSchedule{
		Mode:      mode,
		StartTime: result.StartTime,
		Result:    result,
	}

	// Persist schedule to HASS entities (this also sets the display fields)
	if err := c.stateManager.SaveSchedule(c.pendingSchedule); err != nil {
		log.Printf("ERROR: Failed to save schedule state: %v", err)
		// Continue anyway - schedule is still in memory
	}

	log.Printf("Dishwasher scheduled successfully!")

	// Announce delayed start via TTS
	c.announceDelayedStart(result.StartTime, result.SavingsPercent)
}

// checkPendingStart runs periodically via interval to check if it's time to start
func (c *Dishwasher) checkPendingStart(service *ga.Service, state ga.State) {
	if c.pendingSchedule == nil {
		return
	}

	// Check if user manually cancelled the schedule
	cancelled, err := c.stateManager.IsScheduleCancelled()
	if err != nil {
		log.Printf("ERROR: Failed to check if schedule was cancelled: %v", err)
	} else if cancelled {
		c.cancelPendingSchedule("scheduled flag turned off")
		return
	}

	now := time.Now()
	if now.After(c.pendingSchedule.StartTime) || now.Equal(c.pendingSchedule.StartTime) {
		log.Printf("Time to start dishwasher!")

		if err := c.controller.StartDishwasher(); err != nil {
			log.Printf("ERROR: Failed to start: %v", err)
			return
		}

		// Clear schedule from HASS entities
		if err := c.stateManager.ClearSchedule(); err != nil {
			log.Printf("ERROR: Failed to clear schedule state: %v", err)
		}

		// Clear pending schedule from memory
		c.pendingSchedule = nil
	}
}

// announceDelayedStart fires a notification event for a scheduled dishwasher start
func (c *Dishwasher) announceDelayedStart(startTime time.Time, savingsPercent float64) {
	if c.notificationService == nil {
		return
	}

	// Format time in a natural way for speech
	// e.g., "3 PM", "3:30 PM", "noon", "midnight"
	timeStr := notifications.FormatTimeForSpeech(startTime)

	message := fmt.Sprintf(
		"Dishwasher starts at %s, saving %.0f percent on electricity!",
		timeStr,
		savingsPercent,
	)

	event := notifications.NotificationEvent{
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

	timeStr := notifications.FormatTimeForSpeech(startTime)

	message := fmt.Sprintf(
		"Dishwasher starts now, saving %.0f percent on electricity!",
		savingsPercent,
	)

	event := notifications.NotificationEvent{
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

	timeStr := notifications.FormatTimeForSpeech(schedule.StartTime)
	message := fmt.Sprintf("Dishwasher schedule for %s was cancelled", timeStr)
	if suffix := cancellationReasonToSpeech(reason); suffix != "" {
		message = fmt.Sprintf("%s %s.", message, suffix)
	} else {
		message += "."
	}

	event := notifications.NotificationEvent{
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

// handleScheduleFlagChange reacts to Home Assistant helper changes
func (c *Dishwasher) handleScheduleFlagChange(service *ga.Service, state ga.State, data ga.EntityData) {
	if data.ToState != "off" {
		return
	}

	if c.pendingSchedule == nil {
		return
	}

	c.cancelPendingSchedule("input_boolean turned off")
}

// cancelPendingSchedule clears local + HA state for a pending run
func (c *Dishwasher) cancelPendingSchedule(reason string) {
	if c.pendingSchedule == nil {
		log.Printf("Cancellation requested (%s) but no pending dishwasher schedule", reason)
	} else {
		log.Printf("Cancelling pending dishwasher schedule (%s)", reason)
		schedule := c.pendingSchedule
		c.pendingSchedule = nil
		c.announceCancellation(schedule, reason)
	}

	if err := c.stateManager.ClearSchedule(); err != nil {
		log.Printf("ERROR: Failed to clear schedule state: %v", err)
	}
}

func cancellationReasonToSpeech(reason string) string {
	switch reason {
	case "cancel event":
		return "after a cancel request"
	case "scheduled flag turned off":
		return "because the schedule toggle was turned off"
	case "input_boolean turned off":
		return "manually from Home Assistant"
	default:
		if reason == "" {
			return ""
		}
		return fmt.Sprintf("(%s)", reason)
	}
}
