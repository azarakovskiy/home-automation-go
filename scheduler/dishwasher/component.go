package dishwasher

import (
	"fmt"
	"log"
	"time"

	"home-go/component"
	"home-go/entities"
	"home-go/notifications"
	"home-go/pricing"
	"home-go/scheduler"
	"home-go/scheduler/optimizer"

	ga "saml.dev/gome-assistant"
)

// Dishwasher handles all dishwasher-related automation
type Dishwasher struct {
	component.Base // Embed Base to get default implementations and common services

	priceService        *pricing.Service
	notificationService *notifications.NotificationService
	controller          *Controller
	optimizer           *optimizer.Optimizer
	stateManager        *StateManager

	pendingSchedule *PendingSchedule
}

// PendingSchedule tracks a scheduled dishwasher cycle
type PendingSchedule struct {
	Mode      Mode
	StartTime time.Time
	Result    *optimizer.OptimizationResult
}

// New creates a new dishwasher component
func New(base component.Base, state ga.State, priceService *pricing.Service) *Dishwasher {
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

// Intervals returns intervals for this component
func (c *Dishwasher) Intervals() []ga.Interval {
	// Check every 5 minutes if it's time to start pending dishwasher
	checkInterval := ga.NewInterval().
		Call(c.checkPendingStart).
		Every("5m").
		Build()

	return []ga.Interval{checkInterval}
}

// Note: EntityListeners() and Schedules() are not defined here.
// They use the default empty implementations from component.Base.

// handleScheduleRequest processes strongly-typed dishwasher schedule events
// The TypedEventHandler automatically parses the event, so we receive typed data directly
func (c *Dishwasher) handleScheduleRequest(service *ga.Service, state ga.State, request scheduler.ScheduleRequest) {
	log.Printf("Received schedule request event")

	// Type-safe access to fields - no parsing needed!
	if request.Device != "dishwasher" {
		log.Printf("Event not for dishwasher: %s", request.Device)
		return
	}

	mode := request.Mode
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

	// Decide: start now or delay?
	delayDuration := time.Until(result.StartTime)
	shouldDelay := c.optimizer.ShouldDelay(result, request.MaxDelayHours) && delayDuration >= 5*time.Minute

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

		// Announce immediate start via TTS
		c.announceImmediateStart(result.SavingsPercent)
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
		log.Printf("Schedule was manually cancelled by user")
		c.pendingSchedule = nil
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

// announceImmediateStart fires a notification event for an immediate dishwasher start
func (c *Dishwasher) announceImmediateStart(savingsPercent float64) {
	message := fmt.Sprintf(
		"Dishwasher starts now, saving %.0f percent on electricity!",
		savingsPercent,
	)

	event := notifications.NotificationEvent{
		Device:  "dishwasher",
		Type:    "started",
		Message: message,
		Data: map[string]interface{}{
			"savings_percent": savingsPercent,
		},
	}

	if err := c.notificationService.Notify(event); err != nil {
		log.Printf("WARNING: Notification event failed: %v", err)
	}
}
