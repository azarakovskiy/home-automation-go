package optimizer

import (
	"fmt"
	"math"
	"time"

	"home-go/pricing"
)

// DeviceProfile defines characteristics of any device cycle
// This is the generic interface that all devices must implement
type DeviceProfile interface {
	// GetDurationHours returns total cycle duration
	GetDurationHours() int

	// GetStageWeights returns importance weights for each stage
	// Sum of weights doesn't need to equal 1.0
	// Higher weight = more important to optimize that stage
	GetStageWeights() []float64

	// GetPowerKW returns average power consumption in kilowatts
	GetPowerKW() float64

	// GetMode returns the device mode identifier
	GetMode() string

	// GetMinSavingsPercent returns minimum savings percentage to delay start
	// e.g., 5.0 means delay only if savings >= 5% of immediate cost
	GetMinSavingsPercent() float64
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
	// No fixed threshold - each device profile specifies its own minimum savings percentage
}

// NewOptimizer creates a new generic optimizer
func NewOptimizer() *Optimizer {
	return &Optimizer{}
}

// Optimize finds the best start time for a device cycle
func (o *Optimizer) Optimize(req OptimizationRequest) (*OptimizationResult, error) {
	if len(req.PriceSlots) == 0 {
		return nil, fmt.Errorf("no price slots available")
	}

	if req.Profile == nil {
		return nil, fmt.Errorf("device profile is required")
	}

	// Calculate slot duration (works for both 1h and 15m intervals)
	slotDuration := req.PriceSlots[0].Till.Sub(req.PriceSlots[0].From)

	// Calculate how many slots needed for the cycle
	cycleDuration := time.Duration(req.Profile.GetDurationHours()) * time.Hour
	slotsNeeded := int(math.Ceil(float64(cycleDuration) / float64(slotDuration)))

	// If we don't have enough data, optimize with what we have and start immediately
	availableSlots := len(req.PriceSlots)
	if slotsNeeded > availableSlots {
		// Use all available slots - we'll optimize for partial data
		slotsNeeded = availableSlots
	}

	// Calculate deadline
	now := time.Now()
	if req.StartAfter != nil && req.StartAfter.After(now) {
		now = *req.StartAfter
	}
	deadline := now.Add(time.Duration(req.MaxDelayHours) * time.Hour)

	// Try each possible start slot within deadline
	var bestResult *OptimizationResult
	bestWeightedCost := math.MaxFloat64

	for startIdx := 0; startIdx <= len(req.PriceSlots)-slotsNeeded; startIdx++ {
		startSlot := req.PriceSlots[startIdx]

		// Skip if before allowed start time
		if startSlot.From.Before(now) {
			continue
		}

		// Check if this start time respects the deadline
		endTime := startSlot.From.Add(cycleDuration)
		if endTime.After(deadline) {
			break // Can't start any later
		}

		// Calculate cost for this start time
		result := o.calculateCostForWindow(
			req.Profile,
			req.PriceSlots[startIdx:startIdx+slotsNeeded],
			slotDuration,
		)
		result.StartTime = startSlot.From
		result.EndTime = endTime

		if result.WeightedCost < bestWeightedCost {
			bestWeightedCost = result.WeightedCost
			bestResult = result
		}
	}

	if bestResult == nil {
		// No valid optimization window found - start immediately
		// This ensures the device will always run even if we can't optimize
		firstValidIdx := 0
		for i, slot := range req.PriceSlots {
			if !slot.From.Before(now) {
				firstValidIdx = i
				break
			}
		}

		// Create immediate start result with available slots
		slotsToUse := slotsNeeded
		if firstValidIdx+slotsToUse > len(req.PriceSlots) {
			slotsToUse = len(req.PriceSlots) - firstValidIdx
		}

		immediateResult := o.calculateCostForWindow(
			req.Profile,
			req.PriceSlots[firstValidIdx:firstValidIdx+slotsToUse],
			slotDuration,
		)
		immediateResult.StartTime = req.PriceSlots[firstValidIdx].From
		immediateResult.EndTime = immediateResult.StartTime.Add(cycleDuration)
		immediateResult.CurrentCost = immediateResult.EstimatedCost
		immediateResult.Savings = 0
		immediateResult.SavingsPercent = 0

		return immediateResult, nil
	}

	// Calculate cost if started now (for savings comparison)
	firstValidIdx := 0
	for i, slot := range req.PriceSlots {
		if !slot.From.Before(now) {
			firstValidIdx = i
			break
		}
	}

	if firstValidIdx+slotsNeeded <= len(req.PriceSlots) {
		immediateResult := o.calculateCostForWindow(
			req.Profile,
			req.PriceSlots[firstValidIdx:firstValidIdx+slotsNeeded],
			slotDuration,
		)
		bestResult.CurrentCost = immediateResult.EstimatedCost
		bestResult.Savings = immediateResult.EstimatedCost - bestResult.EstimatedCost

		// Calculate savings as percentage of immediate cost
		if immediateResult.EstimatedCost > 0 {
			bestResult.SavingsPercent = (bestResult.Savings / immediateResult.EstimatedCost) * 100.0
		}
	}

	return bestResult, nil
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

// ShouldDelay determines if delaying is worth it based on the device profile's threshold
func (o *Optimizer) ShouldDelay(result *OptimizationResult, profile DeviceProfile) bool {
	minSavingsPercent := profile.GetMinSavingsPercent()
	return result.SavingsPercent >= minSavingsPercent
}
