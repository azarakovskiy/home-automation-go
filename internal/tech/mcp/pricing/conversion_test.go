package pricing

import (
	"math"
	"testing"
	"time"

	domainpricing "home-go/internal/domain/pricing"
)

// makeAdapter creates a ServiceAdapter backed by a domain service with
// pre-loaded slots and a fixed clock at base.
func makeAdapter(t *testing.T, base time.Time, slots []domainpricing.PriceSlot) *ServiceAdapter {
	t.Helper()
	svc := domainpricing.NewService(nil)
	svc.UpdateIndex(slots)
	return NewServiceAdapter(svc, func() time.Time { return base })
}

// makeHourlySlots builds a slot sequence starting at base with one slot per
// hour. The first price in prices corresponds to the slot starting at base.
func makeHourlySlots(base time.Time, prices []float64) []domainpricing.PriceSlot {
	slots := make([]domainpricing.PriceSlot, len(prices))
	for i, p := range prices {
		slots[i] = domainpricing.PriceSlot{
			From:  base.Add(time.Duration(i) * time.Hour),
			Till:  base.Add(time.Duration(i+1) * time.Hour),
			Price: p,
		}
	}
	return slots
}

func TestServiceAdapter_GetCurrentPrice_Delegates(t *testing.T) {
	base := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	slots := makeHourlySlots(base, []float64{0.10, 0.20, 0.30})
	adapter := makeAdapter(t, base.Add(30*time.Minute), slots)

	got, err := adapter.GetCurrentPrice()
	if err != nil {
		t.Fatalf("GetCurrentPrice() error = %v", err)
	}
	if got != 0.10 {
		t.Errorf("GetCurrentPrice() = %v, want 0.10", got)
	}
}

func TestServiceAdapter_GetNextPrice_FirstSlotAtOrAfterNow(t *testing.T) {
	base := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	slots := makeHourlySlots(base, []float64{0.10, 0.20, 0.30, 0.40})
	// "Now" is 10:30 — current slot is 10:00, next slot starts at 11:00.
	adapter := makeAdapter(t, base.Add(30*time.Minute), slots)

	got, err := adapter.GetNextPrice(120)
	if err != nil {
		t.Fatalf("GetNextPrice() error = %v", err)
	}
	if got.Price != 0.20 {
		t.Errorf("Price = %v, want 0.20", got.Price)
	}
	if got.From != "2024-06-01T11:00:00Z" {
		t.Errorf("From = %q, want 2024-06-01T11:00:00Z", got.From)
	}
	if got.Till != "2024-06-01T12:00:00Z" {
		t.Errorf("Till = %q, want 2024-06-01T12:00:00Z", got.Till)
	}
	if got.Level == "" {
		t.Error("Level is empty")
	}
}

func TestServiceAdapter_GetNextPrice_SlotStartingExactlyAtNow(t *testing.T) {
	base := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	slots := makeHourlySlots(base, []float64{0.10, 0.20, 0.30})
	// "Now" is exactly 11:00 — the next slot is the one starting now.
	adapter := makeAdapter(t, base.Add(time.Hour), slots)

	got, err := adapter.GetNextPrice(60)
	if err != nil {
		t.Fatalf("GetNextPrice() error = %v", err)
	}
	if got.Price != 0.20 {
		t.Errorf("Price = %v, want 0.20", got.Price)
	}
}

func TestServiceAdapter_GetNextPrice_EmptyWindow(t *testing.T) {
	base := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	slots := makeHourlySlots(base, []float64{0.10, 0.20})
	// "Now" is at the end of the second slot — nothing ahead within 30 min.
	adapter := makeAdapter(t, base.Add(2*time.Hour), slots)

	_, err := adapter.GetNextPrice(30)
	if err == nil {
		t.Fatal("GetNextPrice() with empty window error = nil, want non-nil")
	}
}

func TestServiceAdapter_GetPriceSummary_BasicFields(t *testing.T) {
	base := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	// 8 hours: cheap, cheap, avg, avg, expensive, expensive, negative, cheap.
	prices := []float64{0.05, 0.06, 0.10, 0.11, 0.30, 0.32, -0.02, 0.04}
	slots := makeHourlySlots(base, prices)
	adapter := makeAdapter(t, base.Add(30*time.Minute), slots)

	got, err := adapter.GetPriceSummary()
	if err != nil {
		t.Fatalf("GetPriceSummary() error = %v", err)
	}
	if got.CurrentPrice != 0.05 {
		t.Errorf("CurrentPrice = %v, want 0.05", got.CurrentPrice)
	}
	if got.MedianPrice == 0 {
		t.Error("MedianPrice = 0, want non-zero")
	}
	if got.MinPrice != -0.02 {
		t.Errorf("MinPrice = %v, want -0.02", got.MinPrice)
	}
	if got.MaxPrice != 0.32 {
		t.Errorf("MaxPrice = %v, want 0.32", got.MaxPrice)
	}
	if got.AveragePrice == 0 {
		t.Error("AveragePrice = 0, want non-zero")
	}
}

func TestServiceAdapter_GetPriceSummary_WindowsHaveAvgPrice(t *testing.T) {
	base := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	prices := []float64{0.05, 0.06, 0.10, 0.11, 0.30, 0.32, -0.02, 0.04}
	slots := makeHourlySlots(base, prices)
	adapter := makeAdapter(t, base.Add(30*time.Minute), slots)

	got, err := adapter.GetPriceSummary()
	if err != nil {
		t.Fatalf("GetPriceSummary() error = %v", err)
	}
	if len(got.CheapWindows) == 0 {
		t.Fatal("CheapWindows is empty, want at least one")
	}
	for _, w := range got.CheapWindows {
		if w.From == "" || w.Till == "" {
			t.Errorf("Window has empty bounds: %+v", w)
		}
	}
	if len(got.NegativeWindows) == 0 {
		t.Error("NegativeWindows is empty, want one entry (the -0.02 slot)")
	}
	if len(got.ExpensiveWindows) == 0 {
		t.Error("ExpensiveWindows is empty, want one entry (the 0.30/0.32 slots)")
	}
}

func TestServiceAdapter_GetPriceSummary_OmitsEmptyNegativeWindows(t *testing.T) {
	base := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	prices := []float64{0.05, 0.06, 0.10, 0.11, 0.30, 0.32, 0.04, 0.03}
	slots := makeHourlySlots(base, prices)
	adapter := makeAdapter(t, base.Add(30*time.Minute), slots)

	got, err := adapter.GetPriceSummary()
	if err != nil {
		t.Fatalf("GetPriceSummary() error = %v", err)
	}
	if got.NegativeWindows != nil {
		t.Errorf("NegativeWindows = %+v, want nil (omitempty when no negatives)", got.NegativeWindows)
	}
}

func TestServiceAdapter_FindCheapestWindow_Success(t *testing.T) {
	base := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	prices := []float64{0.10, 0.05, 0.04, 0.03, 0.30, 0.32, 0.33, 0.31}
	slots := makeHourlySlots(base, prices)
	// "Now" is 10:30, look 2h ahead. The PriceIndex.FindCheapestWindow
	// algorithm includes the partially-passed 10:00-11:00 slot in candidates
	// (it spans 10:00-12:00) — that's the existing domain behaviour. The
	// returned window is 10:00-12:00 with avg (0.10+0.05)/2 = 0.075.
	adapter := makeAdapter(t, base.Add(30*time.Minute), slots)

	got, err := adapter.FindCheapestWindow(120, 120)
	if err != nil {
		t.Fatalf("FindCheapestWindow() error = %v", err)
	}
	if got.From != "2024-06-01T10:00:00Z" {
		t.Errorf("From = %q, want 2024-06-01T10:00:00Z", got.From)
	}
	if got.Till != "2024-06-01T12:00:00Z" {
		t.Errorf("Till = %q, want 2024-06-01T12:00:00Z", got.Till)
	}
	if math.Abs(got.AvgPrice-0.075) > 1e-9 {
		t.Errorf("AvgPrice = %v, want 0.075", got.AvgPrice)
	}
}

func TestServiceAdapter_FindCheapestWindow_NoFit(t *testing.T) {
	base := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
	// All slots start after the 30-minute deadline — no candidates at all.
	slots := makeHourlySlots(base.Add(time.Hour), []float64{0.10, 0.20, 0.30})
	adapter := makeAdapter(t, base, slots)

	_, err := adapter.FindCheapestWindow(60, 30)
	if err == nil {
		t.Fatal("FindCheapestWindow() with no candidates error = nil, want non-nil")
	}
}

func TestServiceAdapter_NilNow_DefaultsToTimeNow(t *testing.T) {
	svc := domainpricing.NewService(nil)
	adapter := NewServiceAdapter(svc, nil)
	if adapter == nil {
		t.Fatal("NewServiceAdapter(nil) = nil, want non-nil")
	}
	if adapter.now == nil {
		t.Error("now function is nil, want default to time.Now")
	}
}

func TestPriceRange_EmptyRange(t *testing.T) {
	slots := makeHourlySlots(time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), []float64{0.10, 0.20})
	from := time.Date(2024, 6, 2, 0, 0, 0, 0, time.UTC)
	deadline := from.Add(time.Hour)
	min, max, avg := priceRange(slots, from, deadline)
	if min != 0 || max != 0 || avg != 0 {
		t.Errorf("priceRange() = (%v, %v, %v), want (0, 0, 0)", min, max, avg)
	}
}

func TestConvertWindows_Empty(t *testing.T) {
	idx := domainpricing.NewPriceIndex(nil)
	got := convertWindows(idx, nil)
	if got != nil {
		t.Errorf("convertWindows(nil) = %+v, want nil", got)
	}
}
