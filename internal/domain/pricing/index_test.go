package pricing

import (
	"testing"
	"time"
)

func makeSlots(base time.Time, prices []float64, slotDur time.Duration) []PriceSlot {
	slots := make([]PriceSlot, len(prices))
	for i, p := range prices {
		slots[i] = PriceSlot{
			From:  base.Add(time.Duration(i) * slotDur),
			Till:  base.Add(time.Duration(i+1) * slotDur),
			Price: p,
		}
	}
	return slots
}

func TestPriceIndex_IsEmpty(t *testing.T) {
	if !NewPriceIndex(nil).IsEmpty() {
		t.Error("empty index should be empty")
	}
	base := time.Now()
	idx := NewPriceIndex(makeSlots(base, []float64{0.1, 0.2}, time.Hour))
	if idx.IsEmpty() {
		t.Error("non-empty index should not be empty")
	}
}

func TestPriceIndex_SlotAt(t *testing.T) {
	base := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	slots := makeSlots(base, []float64{0.10, 0.20, 0.30}, time.Hour)
	idx := NewPriceIndex(slots)

	slot, ok := idx.SlotAt(base.Add(30 * time.Minute))
	if !ok {
		t.Fatal("expected slot at 10:30")
	}
	if slot.Price != 0.10 {
		t.Errorf("expected price 0.10, got %.2f", slot.Price)
	}

	slot, ok = idx.SlotAt(base.Add(90 * time.Minute))
	if !ok {
		t.Fatal("expected slot at 11:30")
	}
	if slot.Price != 0.20 {
		t.Errorf("expected price 0.20, got %.2f", slot.Price)
	}

	_, ok = idx.SlotAt(base.Add(-1 * time.Minute))
	if ok {
		t.Error("expected no slot before first slot")
	}
}

func TestPriceIndex_Level(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	// 10 slots: 4 cheap, 3 average, 3 expensive → thresholds at p35 and p70
	prices := []float64{0.10, 0.12, 0.13, 0.14, 0.20, 0.22, 0.24, 0.35, 0.38, 0.40}
	slots := makeSlots(base, prices, time.Hour)
	idx := NewPriceIndex(slots)

	cheapSlot := slots[0] // 0.10
	highSlot := slots[9]  // 0.40

	if idx.Level(cheapSlot) != PriceLevelCheap {
		t.Errorf("expected cheap level for lowest price slot")
	}
	if idx.Level(highSlot) != PriceLevelHigh {
		t.Errorf("expected high level for highest price slot")
	}
}

func TestPriceIndex_MedianPrice(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	slots := makeSlots(base, []float64{0.10, 0.20, 0.30}, time.Hour)
	idx := NewPriceIndex(slots)

	// Median of [0.10, 0.20, 0.30] = 0.20
	if idx.MedianPrice() != 0.20 {
		t.Errorf("expected median 0.20, got %.2f", idx.MedianPrice())
	}
}

func TestPriceIndex_IsExtreme(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	slots := makeSlots(base, []float64{0.10, 0.20, 0.30}, time.Hour)
	idx := NewPriceIndex(slots)

	spikeSlot := PriceSlot{Price: 0.70} // > 0.20 * 3.0 = 0.60
	normalSlot := PriceSlot{Price: 0.30}
	negSlot := PriceSlot{Price: -0.05}

	if !idx.IsExtreme(spikeSlot, 3.0) {
		t.Error("0.70 should be extreme at 3.0× median (0.60)")
	}
	if idx.IsExtreme(normalSlot, 3.0) {
		t.Error("0.30 should not be extreme at 3.0× median (0.60)")
	}
	if !idx.IsExtreme(negSlot, 3.0) {
		t.Error("negative price should always be extreme")
	}
}

func TestPriceIndex_FindCheapestWindow_1h(t *testing.T) {
	base := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	slots := makeSlots(base, []float64{0.30, 0.15, 0.10, 0.20, 0.25}, time.Hour)
	idx := NewPriceIndex(slots)

	window, ok := idx.FindCheapestWindow(time.Hour, base, base.Add(5*time.Hour))
	if !ok {
		t.Fatal("expected a window to be found")
	}
	// Cheapest single-hour slot is index 2 (0.10)
	if len(window) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(window))
	}
	if window[0].Price != 0.10 {
		t.Errorf("expected cheapest slot price 0.10, got %.2f", window[0].Price)
	}
}

func TestPriceIndex_FindCheapestWindow_2h(t *testing.T) {
	base := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	slots := makeSlots(base, []float64{0.30, 0.15, 0.10, 0.11, 0.25}, time.Hour)
	idx := NewPriceIndex(slots)

	// Cheapest 2h window: slots at index 2+3 → avg (0.10+0.11)/2 = 0.105
	window, ok := idx.FindCheapestWindow(2*time.Hour, base, base.Add(5*time.Hour))
	if !ok {
		t.Fatal("expected a window to be found")
	}
	if len(window) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(window))
	}
	if window[0].Price != 0.10 || window[1].Price != 0.11 {
		t.Errorf("expected window starting at 0.10/0.11 slots, got %.2f/%.2f", window[0].Price, window[1].Price)
	}
}

func TestPriceIndex_FindCheapestWindow_15min(t *testing.T) {
	base := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	// 8 slots of 15 minutes; cheapest 4-slot (1h) window is slots 4-7 (prices 0.10,0.11,0.12,0.10)
	prices := []float64{0.30, 0.28, 0.25, 0.22, 0.10, 0.11, 0.12, 0.10}
	slots := makeSlots(base, prices, 15*time.Minute)
	idx := NewPriceIndex(slots)

	window, ok := idx.FindCheapestWindow(time.Hour, base, base.Add(2*time.Hour))
	if !ok {
		t.Fatal("expected a window to be found")
	}
	if window[0].Price > 0.15 {
		t.Errorf("expected cheap window (avg ≈ 0.108), got first slot price %.2f", window[0].Price)
	}
}

func TestPriceIndex_FindCheapestSlots(t *testing.T) {
	base := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	slots := makeSlots(base, []float64{0.30, 0.10, 0.25, 0.05, 0.20}, time.Hour)
	idx := NewPriceIndex(slots)

	cheapest := idx.FindCheapestSlots(2, base, base.Add(5*time.Hour))
	if len(cheapest) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(cheapest))
	}
	// Cheapest 2: 0.05 and 0.10
	prices := map[float64]bool{0.05: true, 0.10: true}
	for _, s := range cheapest {
		if !prices[s.Price] {
			t.Errorf("unexpected price in cheapest slots: %.2f", s.Price)
		}
	}
}

func TestPriceIndex_HasNegativePrices(t *testing.T) {
	base := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	slots := makeSlots(base, []float64{0.10, -0.05, 0.20}, time.Hour)
	idx := NewPriceIndex(slots)

	if !idx.HasNegativePrices(base, base.Add(3*time.Hour)) {
		t.Error("expected HasNegativePrices = true")
	}
	// Window before negative slot
	if idx.HasNegativePrices(base, base.Add(time.Hour)) {
		t.Error("expected HasNegativePrices = false for first-hour window")
	}
}

func TestPriceIndex_Summary(t *testing.T) {
	base := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	// 6 slots, varied levels
	prices := []float64{0.10, 0.10, 0.20, 0.40, 0.40, -0.05}
	slots := makeSlots(base, prices, time.Hour)
	idx := NewPriceIndex(slots)

	summary := idx.Summary(base, base.Add(6*time.Hour))

	if len(summary.CheapWindows) == 0 {
		t.Error("expected at least one cheap window")
	}
	if len(summary.ExpensiveWindows) == 0 {
		t.Error("expected at least one expensive window")
	}
	if len(summary.NegativeWindows) == 0 {
		t.Error("expected at least one negative window")
	}
	if summary.MedianPrice <= 0 {
		t.Error("expected positive median price")
	}
}
