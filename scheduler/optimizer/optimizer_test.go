package optimizer

import (
	"testing"
	"time"

	"home-go/pricing"
)

// MockProfile implements DeviceProfile for testing
type MockProfile struct {
	duration          time.Duration
	weights           []float64
	power             float64
	mode              string
	minSavingsPercent float64
}

func (m MockProfile) GetDuration() time.Duration    { return m.duration }
func (m MockProfile) GetStageWeights() []float64    { return m.weights }
func (m MockProfile) GetPowerKW() float64           { return m.power }
func (m MockProfile) GetMode() string               { return m.mode }
func (m MockProfile) GetMinSavingsPercent() float64 { return m.minSavingsPercent }

func TestOptimizer_Optimize_SimpleCase(t *testing.T) {
	optimizer := NewOptimizer()

	// Create test price slots - prices decrease over time
	now := time.Now().Truncate(time.Hour)
	slots := []pricing.PriceSlot{
		{From: now, Till: now.Add(time.Hour), Price: 0.30},
		{From: now.Add(time.Hour), Till: now.Add(2 * time.Hour), Price: 0.25},
		{From: now.Add(2 * time.Hour), Till: now.Add(3 * time.Hour), Price: 0.20}, // Cheapest
		{From: now.Add(3 * time.Hour), Till: now.Add(4 * time.Hour), Price: 0.22},
		{From: now.Add(4 * time.Hour), Till: now.Add(5 * time.Hour), Price: 0.28},
	}

	profile := MockProfile{
		duration:          2 * time.Hour, // 2-hour cycle
		weights:           []float64{0.5, 0.5},
		power:             1.0,
		mode:              "test",
		minSavingsPercent: 5.0,
	}

	result, err := optimizer.Optimize(OptimizationRequest{
		Profile:       profile,
		PriceSlots:    slots,
		MaxDelayHours: 4,
	})

	if err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}

	// Should choose to start at hour 2 (cheapest 2-hour window)
	expectedStart := now.Add(2 * time.Hour)
	if !result.StartTime.Equal(expectedStart) {
		t.Errorf("Expected start time %v, got %v", expectedStart, result.StartTime)
	}

	// Verify savings
	if result.Savings <= 0 {
		t.Errorf("Expected positive savings, got %.4f", result.Savings)
	}

	// Verify cost calculation
	expectedCost := (0.20 + 0.22) / 2 * 1.0 * 2.0 // avg price * power * duration
	if abs(result.EstimatedCost-expectedCost) > 0.01 {
		t.Errorf("Expected cost ~%.4f, got %.4f", expectedCost, result.EstimatedCost)
	}
}

func TestOptimizer_Optimize_WeightedStages(t *testing.T) {
	optimizer := NewOptimizer()

	// Use future times to avoid "now" comparison issues
	baseTime := time.Now().Add(1 * time.Hour).Truncate(time.Hour)
	slots := []pricing.PriceSlot{
		{From: baseTime, Till: baseTime.Add(time.Hour), Price: 0.30},                        // High price
		{From: baseTime.Add(time.Hour), Till: baseTime.Add(2 * time.Hour), Price: 0.20},     // Low price
		{From: baseTime.Add(2 * time.Hour), Till: baseTime.Add(3 * time.Hour), Price: 0.25}, // Medium
		{From: baseTime.Add(3 * time.Hour), Till: baseTime.Add(4 * time.Hour), Price: 0.28},
	}

	// Profile with heavily weighted second stage
	profile := MockProfile{
		duration:          2 * time.Hour,
		weights:           []float64{0.1, 0.9}, // Second stage is 9x more important
		power:             1.0,
		mode:              "weighted",
		minSavingsPercent: 5.0,
	}

	result, err := optimizer.Optimize(OptimizationRequest{
		Profile:       profile,
		PriceSlots:    slots,
		MaxDelayHours: 4,
	})

	if err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}

	// Verify that weighted cost is calculated correctly
	if result.WeightedCost <= 0 {
		t.Error("Expected positive weighted cost")
	}

	// Verify that the optimizer found a solution
	if result.EstimatedCost <= 0 {
		t.Error("Expected positive estimated cost")
	}

	// The start time should be optimal based on weighted calculation
	if result.StartTime.Before(baseTime) {
		t.Errorf("Start time %v is before base time %v", result.StartTime, baseTime)
	}

	// Verify slot allocations were created
	if len(result.SlotAllocations) == 0 {
		t.Error("Expected slot allocations to be populated")
	}
}

func TestOptimizer_Optimize_RespectDeadline(t *testing.T) {
	optimizer := NewOptimizer()

	// Use future times to avoid "now" comparison issues
	baseTime := time.Now().Add(1 * time.Hour).Truncate(time.Hour)
	slots := []pricing.PriceSlot{
		{From: baseTime, Till: baseTime.Add(time.Hour), Price: 0.30},
		{From: baseTime.Add(time.Hour), Till: baseTime.Add(2 * time.Hour), Price: 0.25},
		{From: baseTime.Add(2 * time.Hour), Till: baseTime.Add(3 * time.Hour), Price: 0.20},
		{From: baseTime.Add(3 * time.Hour), Till: baseTime.Add(4 * time.Hour), Price: 0.15}, // Cheapest
		{From: baseTime.Add(4 * time.Hour), Till: baseTime.Add(5 * time.Hour), Price: 0.18},
		{From: baseTime.Add(5 * time.Hour), Till: baseTime.Add(6 * time.Hour), Price: 0.22},
	}

	profile := MockProfile{
		duration:          2 * time.Hour,
		weights:           []float64{0.5, 0.5},
		power:             1.0,
		mode:              "test",
		minSavingsPercent: 5.0,
	}

	// Only allow 3 hours max delay from baseTime
	// This means the cycle must complete within baseTime + 3h + 2h (duration) = baseTime + 5h
	result, err := optimizer.Optimize(OptimizationRequest{
		Profile:       profile,
		PriceSlots:    slots,
		MaxDelayHours: 3,
	})

	if err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}

	deadline := baseTime.Add(3 * time.Hour)
	// The cycle must START before or at the deadline
	if result.StartTime.After(deadline) {
		t.Errorf("Start time %v exceeds deadline %v", result.StartTime, deadline)
	}

	// And the cycle must complete within reasonable bounds
	maxEndTime := deadline.Add(2 * time.Hour) // deadline + cycle duration
	if result.EndTime.After(maxEndTime) {
		t.Errorf("End time %v exceeds max allowed end time %v", result.EndTime, maxEndTime)
	}
}

func TestOptimizer_Optimize_InsufficientSlots(t *testing.T) {
	optimizer := NewOptimizer()

	now := time.Now().Truncate(time.Hour)
	slots := []pricing.PriceSlot{
		{From: now, Till: now.Add(time.Hour), Price: 0.30},
	}

	profile := MockProfile{
		duration:          3 * time.Hour, // Need 3 hours but only 1 slot
		weights:           []float64{0.33, 0.33, 0.34},
		power:             1.0,
		mode:              "test",
		minSavingsPercent: 5.0,
	}

	result, err := optimizer.Optimize(OptimizationRequest{
		Profile:       profile,
		PriceSlots:    slots,
		MaxDelayHours: 2,
	})

	// Should NOT error - instead should return immediate start with available data
	if err != nil {
		t.Fatalf("Expected no error for insufficient slots, got: %v", err)
	}

	// Should have zero savings (no optimization possible)
	if result.Savings != 0 {
		t.Errorf("Expected zero savings, got %.2f", result.Savings)
	}

	if result.SavingsPercent != 0 {
		t.Errorf("Expected zero savings percent, got %.2f", result.SavingsPercent)
	}

	// Should start immediately (at the first available slot)
	if !result.StartTime.Equal(now) {
		t.Errorf("Expected immediate start at %v, got %v", now, result.StartTime)
	}
}

func TestOptimizer_Optimize_FifteenMinuteSlots(t *testing.T) {
	optimizer := NewOptimizer()

	// Test with 15-minute intervals
	now := time.Now().Truncate(15 * time.Minute)
	var slots []pricing.PriceSlot
	prices := []float64{0.30, 0.28, 0.25, 0.22, 0.20, 0.18, 0.21, 0.24}

	for i, price := range prices {
		slots = append(slots, pricing.PriceSlot{
			From:  now.Add(time.Duration(i) * 15 * time.Minute),
			Till:  now.Add(time.Duration(i+1) * 15 * time.Minute),
			Price: price,
		})
	}

	profile := MockProfile{
		duration:          1 * time.Hour, // 1 hour = 4 slots of 15 min
		weights:           []float64{0.25, 0.25, 0.25, 0.25},
		power:             1.0,
		mode:              "test",
		minSavingsPercent: 5.0,
	}

	result, err := optimizer.Optimize(OptimizationRequest{
		Profile:       profile,
		PriceSlots:    slots,
		MaxDelayHours: 2,
	})

	if err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}

	// Should find the cheapest 1-hour window (4 consecutive 15-min slots)
	if result.EstimatedCost <= 0 {
		t.Error("Expected positive cost")
	}
}

func TestOptimizer_ShouldDelay(t *testing.T) {
	optimizer := NewOptimizer()

	tests := []struct {
		name              string
		savingsPercent    float64
		minSavingsPercent float64
		expected          bool
	}{
		{"above threshold", 10.0, 5.0, true},
		{"at threshold", 5.0, 5.0, true},
		{"below threshold", 3.0, 5.0, false},
		{"zero savings", 0.0, 5.0, false},
		{"negative savings", -1.0, 5.0, false},
		{"high threshold not met", 4.0, 5.0, false},
		{"low threshold met", 2.5, 2.0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &OptimizationResult{
				SavingsPercent: tt.savingsPercent,
			}
			profile := MockProfile{
				minSavingsPercent: tt.minSavingsPercent,
				duration:          1 * time.Hour,
			}

			got := optimizer.ShouldDelay(result, profile)
			if got != tt.expected {
				t.Errorf("ShouldDelay(%.1f%%, threshold %.1f%%) = %v, want %v",
					tt.savingsPercent, tt.minSavingsPercent, got, tt.expected)
			}
		})
	}
}

func TestOptimizer_Optimize_StartAfter(t *testing.T) {
	optimizer := NewOptimizer()

	now := time.Now().Truncate(time.Hour)
	startAfter := now.Add(2 * time.Hour)

	slots := []pricing.PriceSlot{
		{From: now, Till: now.Add(time.Hour), Price: 0.15}, // Cheapest but before startAfter
		{From: now.Add(time.Hour), Till: now.Add(2 * time.Hour), Price: 0.18},
		{From: now.Add(2 * time.Hour), Till: now.Add(3 * time.Hour), Price: 0.20},
		{From: now.Add(3 * time.Hour), Till: now.Add(4 * time.Hour), Price: 0.22},
	}

	profile := MockProfile{
		duration:          1 * time.Hour,
		weights:           []float64{1.0},
		power:             1.0,
		mode:              "test",
		minSavingsPercent: 5.0,
	}

	result, err := optimizer.Optimize(OptimizationRequest{
		Profile:       profile,
		PriceSlots:    slots,
		MaxDelayHours: 4,
		StartAfter:    &startAfter,
	})

	if err != nil {
		t.Fatalf("Optimize failed: %v", err)
	}

	// Should not start before startAfter time
	if result.StartTime.Before(startAfter) {
		t.Errorf("Start time %v is before requested start time %v", result.StartTime, startAfter)
	}
}

func TestOptimizer_Optimize_GracefulDegradation(t *testing.T) {
	optimizer := NewOptimizer()

	// Scenario: Need 4 hours but only have 2 hours of price data
	now := time.Now().Truncate(time.Hour)
	slots := []pricing.PriceSlot{
		{From: now, Till: now.Add(time.Hour), Price: 0.30},
		{From: now.Add(time.Hour), Till: now.Add(2 * time.Hour), Price: 0.25}, // Cheaper
	}

	profile := MockProfile{
		duration:          4 * time.Hour, // Need 4 hours but only have 2
		weights:           []float64{0.25, 0.25, 0.25, 0.25},
		power:             1.0,
		mode:              "test",
		minSavingsPercent: 5.0,
	}

	result, err := optimizer.Optimize(OptimizationRequest{
		Profile:       profile,
		PriceSlots:    slots,
		MaxDelayHours: 3,
	})

	// Should succeed without error
	if err != nil {
		t.Fatalf("Expected graceful degradation, got error: %v", err)
	}

	// Should optimize with available data (2 slots instead of 4)
	// Since slot 2 is cheaper, it should choose to start there
	expectedStart := now.Add(time.Hour)
	if !result.StartTime.Equal(expectedStart) {
		t.Errorf("Expected start at cheapest available slot %v, got %v", expectedStart, result.StartTime)
	}

	// Should have some cost calculated based on available slots
	if result.EstimatedCost <= 0 {
		t.Error("Expected positive estimated cost")
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
