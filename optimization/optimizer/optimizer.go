package optimizer

import (
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"home-go/internal/domain/pricing"
	"home-go/internal/tech/runtime/debug"
)

var nowFunc = time.Now

// DeviceProfile defines characteristics of any device cycle
// This is the generic interface that all devices must implement
type DeviceProfile interface {
	// GetDuration returns total cycle duration
	GetDuration() time.Duration

	// GetStageWeights returns importance weights for each stage
	// Sum of weights doesn't need to equal 1.0
	// Higher weight = more important to optimize that stage
	GetStageWeights() []float64

	// GetPowerKW returns average power consumption in kilowatts
	GetPowerKW() float64

	// GetMode returns the device mode identifier
	GetMode() string
}

// OptimizationRequest contains all parameters for optimization
type OptimizationRequest struct {
	Profile       DeviceProfile
	PriceSlots    []pricing.PriceSlot
	MaxDelayHours int
	StartAfter    *time.Time // Optional: earliest allowed start
}

// OptimizationResult contains the optimization outcome
type OptimizationResult struct {
	StartTime       time.Time
	EndTime         time.Time
	EstimatedCost   float64
	CurrentCost     float64 // Cost if started immediately
	Savings         float64
	SavingsPercent  float64 // Savings as percentage of current cost
	SlotAllocations []StageAllocation
	WeightedCost    float64
}

// StageAllocation shows which price slots are used for each stage
type StageAllocation struct {
	StageIndex int
	Slots      []pricing.PriceSlot
	Weight     float64
	StageCost  float64
}

// Optimizer is a generic price optimizer for any cyclic device
type Optimizer struct {
	// Dynamic threshold calculation based on MaxDelayHours
	// Reference points:
	// - 12 hours wait → 5% minimum savings
	// - 2 hours wait → 20% minimum savings
	// Inverse relationship: less time = higher threshold
}

// Constants for dynamic threshold calculation using exponential decay
const (
	// Base threshold: asymptotic minimum as delay approaches infinity
	baseThreshold = 2.0

	// Decay rate: controls how quickly threshold decreases with more delay time
	// Higher value = faster decay (threshold drops more quickly)
	decayRate = 0.15

	// Scale factor: multiplier for the exponential term
	// Controls the range of threshold values
	scaleFactor = 25.0
)

// NewOptimizer creates a new generic optimizer
func NewOptimizer() *Optimizer {
	return &Optimizer{}
}

// Optimize finds the best start time for a device cycle
func (o *Optimizer) Optimize(req OptimizationRequest) (*OptimizationResult, error) {
	if err := o.validateRequest(req); err != nil {
		return nil, err
	}

	slotDuration := req.PriceSlots[0].Till.Sub(req.PriceSlots[0].From)
	cycleDuration := req.Profile.GetDuration()
	slotsNeeded := o.calculateSlotsNeeded(cycleDuration, slotDuration, len(req.PriceSlots))

	now := o.calculateStartTime(req.StartAfter)
	deadline := now.Add(time.Duration(req.MaxDelayHours) * time.Hour)

	// Find best optimization window
	bestResult := o.findBestWindow(req, slotsNeeded, slotDuration, cycleDuration, now, deadline)

	if bestResult == nil {
		return o.createImmediateResult(req, slotsNeeded, slotDuration, cycleDuration, now)
	}

	// Calculate savings compared to immediate start
	o.calculateSavings(bestResult, req, slotsNeeded, slotDuration, now)

	return bestResult, nil
}

func (o *Optimizer) validateRequest(req OptimizationRequest) error {
	if len(req.PriceSlots) == 0 {
		return fmt.Errorf("no price slots available")
	}
	if req.Profile == nil {
		return fmt.Errorf("device profile is required")
	}
	return nil
}

func (o *Optimizer) calculateSlotsNeeded(cycleDuration time.Duration, slotDuration time.Duration, availableSlots int) int {
	slotsNeeded := int(math.Ceil(float64(cycleDuration) / float64(slotDuration)))
	if slotsNeeded > availableSlots {
		return availableSlots
	}
	return slotsNeeded
}

func (o *Optimizer) calculateStartTime(startAfter *time.Time) time.Time {
	now := nowFunc()
	if startAfter != nil && startAfter.After(now) {
		return *startAfter
	}
	return now
}

func (o *Optimizer) findBestWindow(
	req OptimizationRequest,
	slotsNeeded int,
	slotDuration time.Duration,
	cycleDuration time.Duration,
	now time.Time,
	deadline time.Time,
) *OptimizationResult {
	var bestResult *OptimizationResult
	bestWeightedCost := math.MaxFloat64

	debug.Log("Finding best window: now=%s, deadline=%s, slotsNeeded=%d",
		now.Format(time.RFC3339), deadline.Format(time.RFC3339), slotsNeeded)

	for startIdx := 0; startIdx <= len(req.PriceSlots)-slotsNeeded; startIdx++ {
		startSlot := req.PriceSlots[startIdx]

		// Skip slots that are in the past
		if startSlot.From.Before(now) {
			debug.Log("Skipping past slot %d: %s (before now)", startIdx, startSlot.From.Format(time.RFC3339))
			continue
		}

		endTime := startSlot.From.Add(cycleDuration)
		if endTime.After(deadline) {
			debug.Log("Stopping at slot %d: end time %s exceeds deadline", startIdx, endTime.Format(time.RFC3339))
			break
		}

		result := o.calculateCostForWindow(
			req.Profile,
			req.PriceSlots[startIdx:startIdx+slotsNeeded],
			slotDuration,
		)
		result.StartTime = startSlot.From
		result.EndTime = endTime

		debug.Log("Evaluated slot %d: start=%s, weightedCost=%.4f, estimatedCost=%.4f",
			startIdx, startSlot.From.Format(time.RFC3339), result.WeightedCost, result.EstimatedCost)

		if result.WeightedCost < bestWeightedCost {
			bestWeightedCost = result.WeightedCost
			bestResult = result
			debug.Log("New best result found at %s", startSlot.From.Format(time.RFC3339))
		}
	}

	if bestResult != nil {
		debug.Log("Final best window: start=%s, weightedCost=%.4f",
			bestResult.StartTime.Format(time.RFC3339), bestResult.WeightedCost)
	}

	return bestResult
}

func (o *Optimizer) createImmediateResult(
	req OptimizationRequest,
	slotsNeeded int,
	slotDuration time.Duration,
	cycleDuration time.Duration,
	now time.Time,
) (*OptimizationResult, error) {
	firstValidIdx := o.findFirstValidSlot(req.PriceSlots, now)
	slotsToUse := slotsNeeded
	if firstValidIdx+slotsToUse > len(req.PriceSlots) {
		slotsToUse = len(req.PriceSlots) - firstValidIdx
	}

	result := o.calculateCostForWindow(
		req.Profile,
		req.PriceSlots[firstValidIdx:firstValidIdx+slotsToUse],
		slotDuration,
	)
	result.StartTime = req.PriceSlots[firstValidIdx].From
	result.EndTime = result.StartTime.Add(cycleDuration)
	result.CurrentCost = result.EstimatedCost
	result.Savings = 0
	result.SavingsPercent = 0

	return result, nil
}

func (o *Optimizer) findFirstValidSlot(slots []pricing.PriceSlot, now time.Time) int {
	for i, slot := range slots {
		if !slot.From.Before(now) {
			return i
		}
	}
	return 0
}

func (o *Optimizer) calculateSavings(
	result *OptimizationResult,
	req OptimizationRequest,
	slotsNeeded int,
	slotDuration time.Duration,
	now time.Time,
) {
	firstValidIdx := o.findFirstValidSlot(req.PriceSlots, now)

	if firstValidIdx+slotsNeeded <= len(req.PriceSlots) {
		immediateResult := o.calculateCostForWindow(
			req.Profile,
			req.PriceSlots[firstValidIdx:firstValidIdx+slotsNeeded],
			slotDuration,
		)
		result.CurrentCost = immediateResult.EstimatedCost
		result.Savings = immediateResult.EstimatedCost - result.EstimatedCost

		if immediateResult.EstimatedCost > 0 {
			result.SavingsPercent = (result.Savings / immediateResult.EstimatedCost) * 100.0
		}
	}
}

// calculateCostForWindow calculates weighted cost for a specific time window
func (o *Optimizer) calculateCostForWindow(
	profile DeviceProfile,
	slots []pricing.PriceSlot,
	slotDuration time.Duration,
) *OptimizationResult {

	weights := profile.GetStageWeights()
	powerKW := profile.GetPowerKW()
	numStages := len(weights)

	slotsPerStage := float64(len(slots)) / float64(numStages)

	var weightedCost float64
	var totalCost float64
	var allocations []StageAllocation

	for stageIdx, weight := range weights {
		// Determine which slots belong to this stage
		stageStartSlot := int(float64(stageIdx) * slotsPerStage)
		stageEndSlot := int(float64(stageIdx+1) * slotsPerStage)

		if stageEndSlot > len(slots) {
			stageEndSlot = len(slots)
		}

		// Collect slots for this stage
		stageSlots := slots[stageStartSlot:stageEndSlot]

		// Calculate average price for this stage
		var stagePrice float64
		for _, slot := range stageSlots {
			stagePrice += slot.Price
		}
		if len(stageSlots) > 0 {
			stagePrice /= float64(len(stageSlots))
		}

		// Calculate costs
		stageDuration := float64(len(stageSlots)) * slotDuration.Hours()
		stageCost := stagePrice * powerKW * stageDuration

		totalCost += stageCost
		weightedCost += stageCost * weight // Weight important stages more heavily

		allocations = append(allocations, StageAllocation{
			StageIndex: stageIdx,
			Slots:      stageSlots,
			Weight:     weight,
			StageCost:  stageCost,
		})
	}

	return &OptimizationResult{
		EstimatedCost:   totalCost,
		WeightedCost:    weightedCost,
		SlotAllocations: allocations,
	}
}

// ShouldDelay determines if delaying is worth it based on dynamic threshold
// The threshold uses an exponential decay function based on MaxDelayHours:
// threshold = baseThreshold + scaleFactor * exp(-decayRate * hours)
//
// This creates a smooth curve where:
// - Short delays (1-2h) require high savings (~20-25%)
// - Medium delays (6h) require moderate savings (~10%)
// - Long delays (12h+) require lower savings (~5%)
// - Very long delays approach base threshold asymptotically (~2%)
func (o *Optimizer) ShouldDelay(result *OptimizationResult, maxDelayHours int) bool {
	if maxDelayHours <= 0 {
		return false
	}

	// Calculate dynamic threshold based on available delay time
	threshold := o.CalculateDynamicThreshold(maxDelayHours)

	debug.Log("ShouldDelay: savings=%.1f%%, threshold=%.1f%% (for %dh delay)",
		result.SavingsPercent, threshold, maxDelayHours)

	return result.SavingsPercent >= threshold
}

// CalculateDynamicThreshold computes the minimum savings threshold using exponential decay
// Formula: threshold = baseThreshold + scaleFactor * exp(-decayRate * hours)
//
// This creates a smooth inverse relationship:
// - 1h delay  → ~23% threshold (very high, need significant savings for short wait)
// - 2h delay  → ~18% threshold
// - 3h delay  → ~15% threshold
// - 6h delay  → ~9% threshold
// - 12h delay → ~5% threshold (willing to wait longer for smaller savings)
// - 24h delay → ~3% threshold (approaches baseThreshold asymptotically)
//
// This is exported so callers can log the threshold value
func (o *Optimizer) CalculateDynamicThreshold(maxDelayHours int) float64 {
	if maxDelayHours <= 0 {
		// For zero or negative delay, return a very high threshold
		// (effectively: don't delay unless savings are exceptional)
		return 100.0
	}

	// Exponential decay function: threshold decreases smoothly as delay time increases
	// base + scale * e^(-decay * hours)
	threshold := baseThreshold + scaleFactor*math.Exp(-decayRate*float64(maxDelayHours))

	return threshold
}

// CheapestHoursRequest contains parameters for simple cheapest-hours optimization
// Used by devices that just need to know: "should I run now?"
// Strategy is auto-detected: if CriticalHoursStart/End are set, uses critical_uptime strategy
type CheapestHoursRequest struct {
	DeviceName    string        // Name of the device for logging (e.g., "Laptop", "Vacuum")
	TotalDuration time.Duration // Total duration needed (e.g., 6h, 1h30m, 45m)
	WindowSize    time.Duration // Time window to search within (e.g., 12h, 8h)

	// For critical_uptime strategy (auto-detected if these are non-zero):
	CriticalHoursStart int           // Hour when device must be available (e.g., 10)
	CriticalHoursEnd   int           // Hour when critical period ends (e.g., 18)
	DrainRate          time.Duration // How long device runs on full charge (e.g., 2h)
	BatteryEntity      string        // Optional: HA entity for battery level
	MinBatteryPercent  int           // Charge during critical hours if battery < this (default: 20)

	// Runtime state (passed by component, not from profile):
	CurrentBatteryLevel int // Current battery %, 0-100. If 0, will estimate based on DrainRate
}

// CheapestHoursResult contains the optimization decision for simple on/off devices
type CheapestHoursResult struct {
	ChargeNow      bool
	CheapestSlots  []pricing.PriceSlot // The cheapest time slots
	AveragePrice   float64
	CurrentPrice   float64
	SavingsPercent float64
	TotalDuration  time.Duration // Actual total duration covered by selected slots
}

// OptimizeCheapestHours determines if device should run now based on cheapest time slots
// Works with any time granularity (15min, 1h, etc.) - just selects cheapest slots until duration is met
//
// Supports two strategies (auto-detected):
// - "opportunistic": Simply charge during cheapest slots (CriticalHours not set)
// - "critical_uptime": Ensure charged before critical hours, skip charging during expensive critical hours
//
// Perfect for chargers, heaters, or any device that can be turned on/off per pricing interval
func (o *Optimizer) OptimizeCheapestHours(req CheapestHoursRequest, priceSlots []pricing.PriceSlot) (*CheapestHoursResult, error) {
	if err := o.validateCheapestHoursRequest(req); err != nil {
		return nil, err
	}

	// Auto-detect strategy based on CriticalHours fields
	hasCriticalHours := req.CriticalHoursStart > 0 || req.CriticalHoursEnd > 0

	if hasCriticalHours {
		return o.optimizeCriticalUptime(req, priceSlots)
	}

	return o.optimizeOpportunistic(req, priceSlots)
}

// optimizeOpportunistic implements simple cheapest-slots strategy
// Just charges during the cheapest available slots, no special logic
func (o *Optimizer) optimizeOpportunistic(req CheapestHoursRequest, priceSlots []pricing.PriceSlot) (*CheapestHoursResult, error) {
	now := nowFunc()
	currentPrice := o.getCurrentPriceFromSlots(priceSlots, now)

	// Filter slots within window
	windowSlots := o.getWindowSlots(priceSlots, now, req.WindowSize)
	if len(windowSlots) == 0 {
		return nil, fmt.Errorf("no price slots available in window")
	}

	debug.Log("Cheapest hours optimization: need %s in %s window, found %d slots",
		req.TotalDuration, req.WindowSize, len(windowSlots))

	// Find cheapest slots until we meet the duration requirement
	cheapestSlots, totalDuration := o.selectCheapestSlotsByDuration(windowSlots, req.TotalDuration)
	currentSlotIsCheap := o.isCurrentSlotInSelection(cheapestSlots, now)

	// Calculate metrics
	avgPrice := o.calculateAveragePrice(windowSlots)
	savingsPercent := o.calculateSavingsPercent(avgPrice, currentPrice)

	debug.Log("Cheapest hours decision: charge now=%v, current=%.4f, avg=%.4f, savings=%.1f%%, selected duration=%s",
		currentSlotIsCheap, currentPrice, avgPrice, savingsPercent, totalDuration)

	// Log user-friendly information
	if !currentSlotIsCheap && len(cheapestSlots) > 0 {
		nextSlot := cheapestSlots[0]
		log.Printf("%s: Not charging now - next cheap slot at %s (avg price: %.2f)",
			req.DeviceName, nextSlot.From.Format("15:04"), avgPrice)
	} else if !currentSlotIsCheap {
		log.Printf("%s: Not charging now - no optimal slots found", req.DeviceName)
	} else {
		log.Printf("%s: Charging now - current slot is cheap (savings: %.1f%%, duration: %s)",
			req.DeviceName, savingsPercent, totalDuration)
	}

	return &CheapestHoursResult{
		ChargeNow:      currentSlotIsCheap,
		CheapestSlots:  cheapestSlots,
		AveragePrice:   avgPrice,
		CurrentPrice:   currentPrice,
		SavingsPercent: savingsPercent,
		TotalDuration:  totalDuration,
	}, nil
}

// optimizeCriticalUptime implements strategy for devices that need to be available during specific hours
// Ensures device is charged before critical hours start, allows drain during expensive critical periods
// Uses actual battery level if available, otherwise estimates based on DrainRate
func (o *Optimizer) optimizeCriticalUptime(req CheapestHoursRequest, priceSlots []pricing.PriceSlot) (*CheapestHoursResult, error) {
	now := nowFunc()
	currentHour := now.Hour()
	currentPrice := o.getCurrentPriceFromSlots(priceSlots, now)

	// Check if we're in critical hours (device is being used, might be draining)
	// Handle both normal range (e.g., 10-18) and wrap-around (e.g., 22-6)
	inCriticalHours := false
	if req.CriticalHoursStart < req.CriticalHoursEnd {
		inCriticalHours = currentHour >= req.CriticalHoursStart && currentHour < req.CriticalHoursEnd
	} else {
		// Wrap-around case: e.g., 22-6 means 22,23,0,1,2,3,4,5
		inCriticalHours = currentHour >= req.CriticalHoursStart || currentHour < req.CriticalHoursEnd
	}

	if inCriticalHours {
		return o.handleCriticalHoursCharging(req, priceSlots, currentPrice)
	}

	// Outside critical hours: charge opportunistically during cheapest slots
	return o.handlePreChargingBeforeCriticalHours(req, priceSlots, now, currentPrice)
}

// handleCriticalHoursCharging decides whether to charge during critical hours
// Uses actual battery sensor if available, otherwise estimates based on drain rate
func (o *Optimizer) handleCriticalHoursCharging(req CheapestHoursRequest, priceSlots []pricing.PriceSlot, currentPrice float64) (*CheapestHoursResult, error) {
	batteryLevel := req.CurrentBatteryLevel

	// If no battery sensor or it might be stale (laptop lid closed), use time-based estimation
	// Battery sensor is considered if > 0, but we should be conservative
	if batteryLevel > 0 {
		return o.handleCriticalHoursWithBatterySensor(req, priceSlots, batteryLevel, currentPrice)
	}

	// Fallback to time-based estimation when sensor unavailable
	return o.handleCriticalHoursWithEstimation(req, currentPrice)
}

// handleCriticalHoursWithBatterySensor handles charging during critical hours when battery sensor is available
func (o *Optimizer) handleCriticalHoursWithBatterySensor(req CheapestHoursRequest, priceSlots []pricing.PriceSlot, batteryLevel int, currentPrice float64) (*CheapestHoursResult, error) {
	now := nowFunc()
	currentHour := now.Hour()
	isWeekend := now.Weekday() == time.Saturday || now.Weekday() == time.Sunday

	// Calculate hours remaining in critical period
	hoursRemaining := req.CriticalHoursEnd - currentHour
	if hoursRemaining < 0 {
		hoursRemaining = 0
	}

	// Emergency charge if battery is critically low
	if batteryLevel < req.MinBatteryPercent {
		log.Printf("%s: CRITICAL - Battery at %d%% (below %d%%), charging despite expensive rates",
			req.DeviceName, batteryLevel, req.MinBatteryPercent)
		return &CheapestHoursResult{
			ChargeNow:      true,
			CheapestSlots:  []pricing.PriceSlot{},
			AveragePrice:   currentPrice,
			CurrentPrice:   currentPrice,
			SavingsPercent: 0,
			TotalDuration:  req.TotalDuration,
		}, nil
	}

	// Weekdays (Mon-Fri): Smart charging based on actual prices, not fixed time slots
	// Weekends (Sat-Sun): Allow aggressive drain to save money, only charge when necessary
	if !isWeekend {
		// Look at ALL available future price data (12-24h) to define what "cheap" means today
		minPrice, avgPrice, maxPrice := o.getPriceStatistics(priceSlots, now)

		if minPrice == 0 || maxPrice == 0 {
			log.Printf("%s: [Weekday] No future price data available", req.DeviceName)
			return &CheapestHoursResult{
				ChargeNow:      false,
				CheapestSlots:  []pricing.PriceSlot{},
				AveragePrice:   currentPrice,
				CurrentPrice:   currentPrice,
				SavingsPercent: 0,
				TotalDuration:  0,
			}, nil
		}

		// Calculate dynamic thresholds based on today's price distribution
		// "Cheap" = min + 25% of range (bottom quartile)
		// "Very cheap" = min + 15% of range (bottom 15%)
		priceRange := maxPrice - minPrice
		cheapThreshold := minPrice + (priceRange * 0.25)
		veryCheapThreshold := minPrice + (priceRange * 0.15)

		// Weekday logic: Charge based on battery level and price
		if batteryLevel < 60 {
			// Low battery (< 60%): charge when price is "cheap"
			if currentPrice <= cheapThreshold {
				savingsPercent := ((avgPrice - currentPrice) / avgPrice) * 100
				log.Printf("%s: [Weekday] Charging - battery at %d%%, cheap price %.4f (threshold: %.4f, range: %.4f-%.4f)",
					req.DeviceName, batteryLevel, currentPrice, cheapThreshold, minPrice, maxPrice)
				return &CheapestHoursResult{
					ChargeNow:      true,
					CheapestSlots:  []pricing.PriceSlot{},
					AveragePrice:   avgPrice,
					CurrentPrice:   currentPrice,
					SavingsPercent: savingsPercent,
					TotalDuration:  req.TotalDuration,
				}, nil
			}

			log.Printf("%s: [Weekday] Battery at %d%%, waiting for cheap price (current: %.4f, cheap: %.4f)",
				req.DeviceName, batteryLevel, currentPrice, cheapThreshold)
		} else {
			// Healthy battery (>= 60%): only charge when price is "very cheap"
			if currentPrice <= veryCheapThreshold {
				savingsPercent := ((avgPrice - currentPrice) / avgPrice) * 100
				log.Printf("%s: [Weekday] Charging - battery at %d%%, very cheap price %.4f (threshold: %.4f, range: %.4f-%.4f)",
					req.DeviceName, batteryLevel, currentPrice, veryCheapThreshold, minPrice, maxPrice)
				return &CheapestHoursResult{
					ChargeNow:      true,
					CheapestSlots:  []pricing.PriceSlot{},
					AveragePrice:   avgPrice,
					CurrentPrice:   currentPrice,
					SavingsPercent: savingsPercent,
					TotalDuration:  req.TotalDuration,
				}, nil
			}

			log.Printf("%s: [Weekday] Battery at %d%%, healthy - waiting for very cheap price (current: %.4f, threshold: %.4f)",
				req.DeviceName, batteryLevel, currentPrice, veryCheapThreshold)
		}
	} else {
		// Weekend logic: Allow aggressive drain to save money
		if req.DrainRate > 0 && hoursRemaining > 0 {
			drainPerHour := 100.0 / (req.DrainRate.Hours())
			estimatedBatteryAtEnd := float64(batteryLevel) - (drainPerHour * float64(hoursRemaining))

			if estimatedBatteryAtEnd < float64(req.MinBatteryPercent) {
				log.Printf("%s: [Weekend] Charging - battery at %d%%, will drop to %.0f%% in %dh",
					req.DeviceName, batteryLevel, estimatedBatteryAtEnd, hoursRemaining)
				return &CheapestHoursResult{
					ChargeNow:      true,
					CheapestSlots:  []pricing.PriceSlot{},
					AveragePrice:   currentPrice,
					CurrentPrice:   currentPrice,
					SavingsPercent: 0,
					TotalDuration:  req.TotalDuration,
				}, nil
			}
		}

		log.Printf("%s: [Weekend] In critical hours, battery at %d%%, allowing drain",
			req.DeviceName, batteryLevel)
	}

	return &CheapestHoursResult{
		ChargeNow:      false,
		CheapestSlots:  []pricing.PriceSlot{},
		AveragePrice:   currentPrice,
		CurrentPrice:   currentPrice,
		SavingsPercent: 0,
		TotalDuration:  0,
	}, nil
}

// handleCriticalHoursWithEstimation handles charging during critical hours using drain rate estimation
func (o *Optimizer) handleCriticalHoursWithEstimation(req CheapestHoursRequest, currentPrice float64) (*CheapestHoursResult, error) {
	now := nowFunc()
	currentHour := now.Hour()
	hoursIntoCritical := currentHour - req.CriticalHoursStart
	criticalHoursRemaining := req.CriticalHoursEnd - currentHour

	if req.DrainRate > 0 {
		// Rough estimate: charge if we've been in critical hours for > 50% of DrainRate
		if time.Duration(hoursIntoCritical)*time.Hour > req.DrainRate/2 {
			log.Printf("%s: %dh into critical period (drain rate: %s), charging to prevent depletion",
				req.DeviceName, hoursIntoCritical, req.DrainRate)
			return &CheapestHoursResult{
				ChargeNow:      true,
				CheapestSlots:  []pricing.PriceSlot{},
				AveragePrice:   currentPrice,
				CurrentPrice:   currentPrice,
				SavingsPercent: 0,
				TotalDuration:  req.TotalDuration,
			}, nil
		}
	}

	log.Printf("%s: In critical hours (%d-%d), %dh remaining, skipping charge",
		req.DeviceName, req.CriticalHoursStart, req.CriticalHoursEnd, criticalHoursRemaining)

	return &CheapestHoursResult{
		ChargeNow:      false,
		CheapestSlots:  []pricing.PriceSlot{},
		AveragePrice:   currentPrice,
		CurrentPrice:   currentPrice,
		SavingsPercent: 0,
		TotalDuration:  0,
	}, nil
}

// handlePreChargingBeforeCriticalHours optimizes charging outside critical hours to prepare for the day
func (o *Optimizer) handlePreChargingBeforeCriticalHours(req CheapestHoursRequest, priceSlots []pricing.PriceSlot, now time.Time, currentPrice float64) (*CheapestHoursResult, error) {
	windowSlots := o.getWindowSlots(priceSlots, now, req.WindowSize)
	if len(windowSlots) == 0 {
		return nil, fmt.Errorf("no price slots available in window")
	}

	debug.Log("Critical uptime optimization: need %s before %d:00, found %d slots",
		req.TotalDuration, req.CriticalHoursStart, len(windowSlots))

	// Find cheapest slots for charging
	cheapestSlots, totalDuration := o.selectCheapestSlotsByDuration(windowSlots, req.TotalDuration)
	currentSlotIsCheap := o.isCurrentSlotInSelection(cheapestSlots, now)

	// Check if we should charge now to avoid expensive critical hours
	shouldChargeNow, criticalHoursAvg := o.shouldPreChargeNow(req, priceSlots, now, currentPrice, currentSlotIsCheap)

	avgPrice := o.calculateAveragePrice(windowSlots)
	savingsPercent := o.calculateSavingsPercent(avgPrice, currentPrice)

	o.logPreChargeDecision(req, currentPrice, cheapestSlots, shouldChargeNow, currentSlotIsCheap, criticalHoursAvg)

	return &CheapestHoursResult{
		ChargeNow:      shouldChargeNow,
		CheapestSlots:  cheapestSlots,
		AveragePrice:   avgPrice,
		CurrentPrice:   currentPrice,
		SavingsPercent: savingsPercent,
		TotalDuration:  totalDuration,
	}, nil
}

// shouldPreChargeNow determines if we should charge now based on price spikes and critical hours proximity
// Returns the decision and the critical hours average price for logging
func (o *Optimizer) shouldPreChargeNow(req CheapestHoursRequest, priceSlots []pricing.PriceSlot, now time.Time, currentPrice float64, currentSlotIsCheap bool) (bool, float64) {
	// Always charge if current slot is among the cheapest
	if currentSlotIsCheap {
		return true, 0
	}

	// Check if we're approaching a price spike during critical hours
	criticalHoursAvg := o.getAveragePriceDuringCriticalHours(priceSlots, now, req.CriticalHoursStart, req.CriticalHoursEnd)
	hoursUntilCritical := o.calculateHoursUntilCritical(now, req.CriticalHoursStart)

	// Strategy: Charge whenever current price is significantly cheaper than critical hours
	// This ensures we pre-charge during cheap night hours even if not THE absolute cheapest slot
	// Use a 20% threshold: charge if current price is at least 20% cheaper than critical hours average
	priceRatio := currentPrice / criticalHoursAvg
	significantlyCheaper := priceRatio < 0.80 // Current is 20%+ cheaper

	// Emergency pre-charge if very close to critical hours (≤4h) and any amount cheaper
	closeToCriticalHours := hoursUntilCritical <= 4
	currentCheaperThanCriticalHours := currentPrice < criticalHoursAvg
	shouldEmergencyPreCharge := currentCheaperThanCriticalHours && closeToCriticalHours

	// Charge if either: significantly cheaper OR emergency pre-charge
	shouldCharge := significantlyCheaper || shouldEmergencyPreCharge

	debug.Log("Pre-charge decision: current=%.4f, critical_avg=%.4f, hours_until_critical=%d, ratio=%.2f, significantly_cheaper=%v, emergency=%v",
		currentPrice, criticalHoursAvg, hoursUntilCritical, priceRatio, significantlyCheaper, shouldEmergencyPreCharge)

	return shouldCharge, criticalHoursAvg
}

// calculateHoursUntilCritical calculates hours until critical hours start, handling day wrap-around
func (o *Optimizer) calculateHoursUntilCritical(now time.Time, criticalHoursStart int) int {
	currentHour := now.Hour()
	hoursUntil := criticalHoursStart - currentHour
	if hoursUntil < 0 {
		hoursUntil += 24 // Handle wrap-around (e.g., 23:00 to 01:00 = 2 hours)
	}
	return hoursUntil
}

// logPreChargeDecision logs user-friendly information about the pre-charge decision
func (o *Optimizer) logPreChargeDecision(req CheapestHoursRequest, currentPrice float64, cheapestSlots []pricing.PriceSlot, shouldChargeNow bool, currentSlotIsCheap bool, criticalHoursAvg float64) {
	if !shouldChargeNow {
		if len(cheapestSlots) > 0 {
			nextSlot := cheapestSlots[0]
			log.Printf("%s: Not charging now - next cheap slot at %s before critical hours",
				req.DeviceName, nextSlot.From.Format("15:04"))
		} else {
			log.Printf("%s: Not charging now - waiting for cheaper slots before %d:00",
				req.DeviceName, req.CriticalHoursStart)
		}
		return
	}

	// Already charging and it's among cheapest slots - no special logging needed
	if currentSlotIsCheap {
		return
	}

	// Emergency pre-charge due to approaching spike
	savingsVsCritical := ((criticalHoursAvg - currentPrice) / criticalHoursAvg) * 100
	log.Printf("%s: Pre-charging now - %.1f%% cheaper than critical hours peak (%.4f vs %.4f)",
		req.DeviceName, savingsVsCritical, currentPrice, criticalHoursAvg)
}

func (o *Optimizer) validateCheapestHoursRequest(req CheapestHoursRequest) error {
	if req.TotalDuration <= 0 {
		return fmt.Errorf("total_duration must be positive")
	}
	if req.WindowSize <= 0 {
		return fmt.Errorf("window_size must be positive")
	}
	if req.TotalDuration > req.WindowSize {
		return fmt.Errorf("total_duration (%s) cannot exceed window_size (%s)",
			req.TotalDuration, req.WindowSize)
	}
	return nil
}

func (o *Optimizer) getWindowSlots(priceSlots []pricing.PriceSlot, now time.Time, windowSize time.Duration) []pricing.PriceSlot {
	windowEnd := now.Add(windowSize)
	var windowSlots []pricing.PriceSlot
	for _, slot := range priceSlots {
		// Include current slot if it overlaps with now
		if (slot.From.Before(now) || slot.From.Equal(now)) && slot.Till.After(now) {
			windowSlots = append(windowSlots, slot)
		} else if slot.From.After(now) && slot.From.Before(windowEnd) {
			windowSlots = append(windowSlots, slot)
		}
	}
	return windowSlots
}

// getPriceStatistics calculates min, avg, max for ALL future price slots
// This defines what "cheap" and "expensive" mean today
func (o *Optimizer) getPriceStatistics(priceSlots []pricing.PriceSlot, now time.Time) (float64, float64, float64) {
	var minPrice, avgPrice, maxPrice float64
	var totalPrice float64
	var count int
	foundAny := false

	for _, slot := range priceSlots {
		// Include all future slots (not just critical hours)
		if slot.From.After(now) || slot.From.Equal(now) {
			if !foundAny {
				minPrice = slot.Price
				maxPrice = slot.Price
				foundAny = true
			} else {
				if slot.Price < minPrice {
					minPrice = slot.Price
				}
				if slot.Price > maxPrice {
					maxPrice = slot.Price
				}
			}
			totalPrice += slot.Price
			count++
		}
	}

	if count > 0 {
		avgPrice = totalPrice / float64(count)
	}

	return minPrice, avgPrice, maxPrice
}

// getAveragePriceDuringCriticalHours calculates average price during critical hours to detect spikes
func (o *Optimizer) getAveragePriceDuringCriticalHours(priceSlots []pricing.PriceSlot, now time.Time, startHour, endHour int) float64 {
	var totalPrice float64
	var count int

	for _, slot := range priceSlots {
		slotHour := slot.From.Hour()

		// Check if this slot falls within critical hours
		// Handle both normal range (e.g., 10-18) and wrap-around (e.g., 22-6)
		inCriticalHours := false
		if startHour < endHour {
			inCriticalHours = slotHour >= startHour && slotHour < endHour
		} else {
			// Wrap-around case: e.g., 22-6 means 22,23,0,1,2,3,4,5
			inCriticalHours = slotHour >= startHour || slotHour < endHour
		}

		if inCriticalHours && (slot.From.After(now) || slot.From.Equal(now)) {
			totalPrice += slot.Price
			count++
		}
	}

	if count == 0 {
		// If no critical hours slots found (e.g., it's evening and critical hours are in past),
		// return current price as fallback
		return o.getCurrentPriceFromSlots(priceSlots, now)
	}

	return totalPrice / float64(count)
}

// selectCheapestSlotsByDuration selects the cheapest slots until the total duration is met
// Returns both the selected slots and the actual total duration covered
func (o *Optimizer) selectCheapestSlotsByDuration(windowSlots []pricing.PriceSlot, targetDuration time.Duration) ([]pricing.PriceSlot, time.Duration) {
	// Sort by price (cheapest first)
	sortedSlots := make([]pricing.PriceSlot, len(windowSlots))
	copy(sortedSlots, windowSlots)
	sort.Slice(sortedSlots, func(i, j int) bool {
		return sortedSlots[i].Price < sortedSlots[j].Price
	})

	var selected []pricing.PriceSlot
	var totalDuration time.Duration

	// Keep adding cheapest slots until we meet the duration requirement
	for _, slot := range sortedSlots {
		slotDuration := slot.Till.Sub(slot.From)
		selected = append(selected, slot)
		totalDuration += slotDuration

		if totalDuration >= targetDuration {
			break
		}
	}

	return selected, totalDuration
}

func (o *Optimizer) isCurrentSlotInSelection(slots []pricing.PriceSlot, now time.Time) bool {
	for _, slot := range slots {
		if o.isCurrentHourSlot(slot, now) {
			return true
		}
	}
	return false
}

func (o *Optimizer) calculateAveragePrice(slots []pricing.PriceSlot) float64 {
	var totalPrice float64
	for _, slot := range slots {
		totalPrice += slot.Price
	}
	return totalPrice / float64(len(slots))
}

func (o *Optimizer) calculateSavingsPercent(avgPrice, currentPrice float64) float64 {
	if avgPrice > 0 {
		return ((avgPrice - currentPrice) / avgPrice) * 100.0
	}
	return 0.0
}

// Helper methods for cheapest hours optimization

func (o *Optimizer) getCurrentPriceFromSlots(priceSlots []pricing.PriceSlot, now time.Time) float64 {
	for _, slot := range priceSlots {
		if o.isCurrentHourSlot(slot, now) {
			return slot.Price
		}
	}
	return 0.0
}

func (o *Optimizer) isCurrentHourSlot(slot pricing.PriceSlot, now time.Time) bool {
	return !slot.From.After(now) && slot.Till.After(now)
}
