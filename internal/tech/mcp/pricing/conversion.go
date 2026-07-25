package pricing

import (
	"fmt"
	"math"
	"time"

	domainpricing "home-go/internal/domain/pricing"
)

// ServiceAdapter implements the MCP PricingService interface by wrapping the
// domain pricing service. It converts domain types (PriceSlot, PriceIndex,
// IndexSummary, PriceLevel) into MCP transport types so the MCP layer never
// imports the domain package.
type ServiceAdapter struct {
	svc *domainpricing.Service
	now func() time.Time
}

// NewServiceAdapter returns an adapter that satisfies the MCP PricingService
// interface. The now function is injectable for tests; pass nil for time.Now.
func NewServiceAdapter(svc *domainpricing.Service, now func() time.Time) *ServiceAdapter {
	if now == nil {
		now = time.Now
	}
	return &ServiceAdapter{svc: svc, now: now}
}

func (a *ServiceAdapter) GetCurrentPrice() (float64, error) {
	// Look up the slot via the index instead of calling svc.GetCurrentPrice():
	// the service's GetCurrentPrice path checks the cache against its own clock
	// (time.Now by default) and tries to refresh from HA state on miss. The
	// adapter already owns a clock we want to use.
	idx, err := a.svc.CurrentIndex()
	if err != nil {
		return 0, err
	}
	slot, ok := idx.SlotAt(a.now())
	if !ok {
		return 0, fmt.Errorf("no slot at current time")
	}
	return slot.Price, nil
}

func (a *ServiceAdapter) GetNextPrice(windowMinutes int) (NextPriceInfo, error) {
	now := a.now()
	deadline := now.Add(time.Duration(windowMinutes) * time.Minute)

	idx, err := a.svc.CurrentIndex()
	if err != nil {
		return NextPriceInfo{}, err
	}

	// Find the first slot whose From is at or after now. Use Slots() rather
	// than GetPriceSlotsInWindow because the latter uses strict "After" and
	// would miss a slot starting exactly at now.
	for _, slot := range idx.Slots() {
		if slot.From.Before(now) {
			continue
		}
		if !slot.From.Before(deadline) {
			break
		}
		return NextPriceInfo{
			Price: slot.Price,
			From:  slot.From.Format(time.RFC3339),
			Till:  slot.Till.Format(time.RFC3339),
			Level: idx.Level(slot).String(),
		}, nil
	}

	return NextPriceInfo{}, fmt.Errorf("no upcoming price within %d minutes", windowMinutes)
}

func (a *ServiceAdapter) GetPriceSummary() (PriceSummary, error) {
	idx, err := a.svc.CurrentIndex()
	if err != nil {
		return PriceSummary{}, err
	}

	now := a.now()
	slots := idx.Slots()
	if len(slots) == 0 {
		return PriceSummary{}, fmt.Errorf("no price slots available")
	}

	// Horizon spans from the current slot through the last known slot.
	deadline := slots[len(slots)-1].Till

	currentSlot, ok := idx.SlotAt(now)
	if !ok {
		return PriceSummary{}, fmt.Errorf("no slot at current time")
	}

	summary := idx.Summary(now, deadline)

	minPrice, maxPrice, avgPrice := priceRange(slots, now, deadline)

	return PriceSummary{
		CurrentPrice:       currentSlot.Price,
		CurrentLevel:       idx.Level(currentSlot).String(),
		MedianPrice:        idx.MedianPrice(),
		CheapThreshold:     idx.CheapThreshold(),
		ExpensiveThreshold: idx.ExpensiveThreshold(),
		MinPrice:           minPrice,
		MaxPrice:           maxPrice,
		AveragePrice:       avgPrice,
		CheapWindows:       convertWindows(idx, summary.CheapWindows),
		ExpensiveWindows:   convertWindows(idx, summary.ExpensiveWindows),
		NegativeWindows:    convertWindows(idx, summary.NegativeWindows),
	}, nil
}

func (a *ServiceAdapter) FindCheapestWindow(durationMinutes, deadlineMinutes int) (CheapestWindow, error) {
	idx, err := a.svc.CurrentIndex()
	if err != nil {
		return CheapestWindow{}, err
	}

	now := a.now()
	deadline := now.Add(time.Duration(deadlineMinutes) * time.Minute)
	duration := time.Duration(durationMinutes) * time.Minute

	slots, ok := idx.FindCheapestWindow(duration, now, deadline)
	if !ok {
		return CheapestWindow{}, fmt.Errorf("no window of %d minutes within %d minutes", durationMinutes, deadlineMinutes)
	}

	var total float64
	for _, s := range slots {
		total += s.Price
	}
	avg := total / float64(len(slots))

	return CheapestWindow{
		From:     slots[0].From.Format(time.RFC3339),
		Till:     slots[len(slots)-1].Till.Format(time.RFC3339),
		AvgPrice: avg,
		Level:    idx.Level(slots[0]).String(),
	}, nil
}

// priceRange computes min, max, and average prices across slots in
// [from, deadline). Returns 0 for all three if the range is empty.
func priceRange(slots []domainpricing.PriceSlot, from, deadline time.Time) (min, max, avg float64) {
	min = math.MaxFloat64
	count := 0
	var total float64
	for _, s := range slots {
		if s.From.Before(from) {
			continue
		}
		if !s.From.Before(deadline) {
			break
		}
		if s.Price < min {
			min = s.Price
		}
		if s.Price > max {
			max = s.Price
		}
		total += s.Price
		count++
	}
	if count == 0 {
		return 0, 0, 0
	}
	return min, max, total / float64(count)
}

// convertWindows turns domain SummaryWindows into MCP PriceWindows,
// computing the average price of the slots that fall inside each window.
func convertWindows(idx domainpricing.PriceIndex, windows []domainpricing.SummaryWindow) []PriceWindow {
	if len(windows) == 0 {
		return nil
	}
	slots := idx.Slots()
	result := make([]PriceWindow, 0, len(windows))
	for _, w := range windows {
		var total float64
		count := 0
		for _, s := range slots {
			if s.From.Before(w.From) {
				continue
			}
			if !s.From.Before(w.Till) {
				break
			}
			total += s.Price
			count++
		}
		avg := 0.0
		if count > 0 {
			avg = total / float64(count)
		}
		result = append(result, PriceWindow{
			From:     w.From.Format(time.RFC3339),
			Till:     w.Till.Format(time.RFC3339),
			AvgPrice: avg,
		})
	}
	return result
}
