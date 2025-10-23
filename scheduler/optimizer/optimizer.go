package optimizer

import (
	"fmt"
	"math"
	"time"

	"home-go/debug"
	"home-go/pricing"
)

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
	now := time.Now()
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
