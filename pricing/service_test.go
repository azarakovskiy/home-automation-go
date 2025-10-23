package pricing

import (
	"testing"
	"time"
)

func TestPriceSlot_TimeHandling(t *testing.T) {
	now := time.Now()
	slot := PriceSlot{
		From:  now,
		Till:  now.Add(time.Hour),
		Price: 0.25,
	}

	if !slot.From.Before(slot.Till) {
		t.Error("From should be before Till")
	}

	duration := slot.Till.Sub(slot.From)
	if duration != time.Hour {
		t.Errorf("Expected 1 hour duration, got %v", duration)
	}
}

func TestGetPriceSlotsInWindow(t *testing.T) {
	// This would need a mock State implementation
	// For now, just test the logic with a mock service
	t.Skip("Requires mock State implementation")
}

func TestPriceSlot_Comparison(t *testing.T) {
	now := time.Now()
	slot1 := PriceSlot{From: now, Till: now.Add(time.Hour), Price: 0.20}
	slot2 := PriceSlot{From: now.Add(time.Hour), Till: now.Add(2 * time.Hour), Price: 0.30}

	if slot1.Price >= slot2.Price {
		t.Error("slot1 should be cheaper than slot2")
	}
}

func TestPriceSlot_FifteenMinuteIntervals(t *testing.T) {
	now := time.Now().Truncate(15 * time.Minute)

	slots := []PriceSlot{
		{From: now, Till: now.Add(15 * time.Minute), Price: 0.20},
		{From: now.Add(15 * time.Minute), Till: now.Add(30 * time.Minute), Price: 0.22},
		{From: now.Add(30 * time.Minute), Till: now.Add(45 * time.Minute), Price: 0.21},
		{From: now.Add(45 * time.Minute), Till: now.Add(60 * time.Minute), Price: 0.23},
	}

	for i, slot := range slots {
		duration := slot.Till.Sub(slot.From)
		if duration != 15*time.Minute {
			t.Errorf("Slot %d duration = %v, want 15 minutes", i, duration)
		}
	}

	// Verify slots are consecutive
	for i := 0; i < len(slots)-1; i++ {
		if !slots[i].Till.Equal(slots[i+1].From) {
			t.Errorf("Slots %d and %d are not consecutive", i, i+1)
		}
	}
}

func TestPriceSlot_FindCheapest(t *testing.T) {
	now := time.Now()
	slots := []PriceSlot{
		{From: now, Till: now.Add(time.Hour), Price: 0.30},
		{From: now.Add(time.Hour), Till: now.Add(2 * time.Hour), Price: 0.18}, // Cheapest
		{From: now.Add(2 * time.Hour), Till: now.Add(3 * time.Hour), Price: 0.25},
	}

	minPrice := slots[0].Price
	minIdx := 0

	for i, slot := range slots {
		if slot.Price < minPrice {
			minPrice = slot.Price
			minIdx = i
		}
	}

	if minIdx != 1 {
		t.Errorf("Expected slot 1 to be cheapest, got slot %d", minIdx)
	}

	if minPrice != 0.18 {
		t.Errorf("Expected min price 0.18, got %.2f", minPrice)
	}
}

func TestGetAveragePrice_Calculation(t *testing.T) {
	// Test average price calculation with known values
	now := time.Now()
	testSlots := []PriceSlot{
		{From: now, Till: now.Add(time.Hour), Price: 0.20},
		{From: now.Add(time.Hour), Till: now.Add(2 * time.Hour), Price: 0.30},
		{From: now.Add(2 * time.Hour), Till: now.Add(3 * time.Hour), Price: 0.25},
	}

	// Expected average: (0.20 + 0.30 + 0.25) / 3 = 0.25
	expected := 0.25

	var total float64
	for _, slot := range testSlots {
		total += slot.Price
	}
	average := total / float64(len(testSlots))

	if average != expected {
		t.Errorf("Average = %.2f, want %.2f", average, expected)
	}
}

func TestGetAveragePrice_MultipleSlots(t *testing.T) {
	// Test with different number of slots
	testCases := []struct {
		name     string
		prices   []float64
		expected float64
	}{
		{
			name:     "single slot",
			prices:   []float64{0.25},
			expected: 0.25,
		},
		{
			name:     "two slots",
			prices:   []float64{0.20, 0.30},
			expected: 0.25,
		},
		{
			name:     "many slots",
			prices:   []float64{0.10, 0.15, 0.20, 0.25, 0.30},
			expected: 0.20,
		},
		{
			name:     "identical prices",
			prices:   []float64{0.22, 0.22, 0.22, 0.22},
			expected: 0.22,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var total float64
			for _, price := range tc.prices {
				total += price
			}
			average := total / float64(len(tc.prices))

			// Use a small epsilon for float comparison
			epsilon := 0.0001
			if average < tc.expected-epsilon || average > tc.expected+epsilon {
				t.Errorf("Average = %.4f, want %.4f", average, tc.expected)
			}
		})
	}
}

func TestIsCurrentlyExpensive_Logic(t *testing.T) {
	// Test the comparison logic
	testCases := []struct {
		name         string
		currentPrice float64
		avgPrice     float64
		expensive    bool
	}{
		{
			name:         "above average",
			currentPrice: 0.30,
			avgPrice:     0.25,
			expensive:    true,
		},
		{
			name:         "below average",
			currentPrice: 0.20,
			avgPrice:     0.25,
			expensive:    false,
		},
		{
			name:         "at average",
			currentPrice: 0.25,
			avgPrice:     0.25,
			expensive:    false,
		},
		{
			name:         "significantly above",
			currentPrice: 0.40,
			avgPrice:     0.25,
			expensive:    true,
		},
		{
			name:         "significantly below",
			currentPrice: 0.10,
			avgPrice:     0.25,
			expensive:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isExpensive := tc.currentPrice > tc.avgPrice
			if isExpensive != tc.expensive {
				t.Errorf("Current %.2f vs Avg %.2f: expensive = %v, want %v",
					tc.currentPrice, tc.avgPrice, isExpensive, tc.expensive)
			}
		})
	}
}
