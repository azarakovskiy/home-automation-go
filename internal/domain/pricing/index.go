package pricing

import (
	"math"
	"sort"
	"time"
)

// SummaryWindow is a consecutive run of same-level slots.
type SummaryWindow struct {
	Level PriceLevel
	From  time.Time
	Till  time.Time
}

// IndexSummary describes the structure of a pricing day.
type IndexSummary struct {
	CheapWindows     []SummaryWindow
	ExpensiveWindows []SummaryWindow
	NegativeWindows  []SummaryWindow
	MedianPrice      float64
}

// PriceIndex is a pure value type for all pricing queries.
// Constructed from a []PriceSlot snapshot; IO-free.
type PriceIndex struct {
	slots          []PriceSlot
	median         float64
	cheapThreshold float64
	expThreshold   float64
}

// NewPriceIndex builds an index from a slot snapshot.
// Thresholds and median are computed once from the full slot set.
func NewPriceIndex(slots []PriceSlot) PriceIndex {
	if len(slots) == 0 {
		return PriceIndex{}
	}
	prices := make([]float64, len(slots))
	for i, s := range slots {
		prices[i] = s.Price
	}
	cheap, exp := ComputeThresholdsFromPrices(prices, CheapPercentile, ExpensivePercentile)
	sorted := make([]float64, len(prices))
	copy(sorted, prices)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	var median float64
	if len(sorted)%2 == 0 {
		median = (sorted[mid-1] + sorted[mid]) / 2
	} else {
		median = sorted[mid]
	}
	cp := make([]PriceSlot, len(slots))
	copy(cp, slots)
	return PriceIndex{slots: cp, median: median, cheapThreshold: cheap, expThreshold: exp}
}

// IsEmpty reports whether the index has no slots.
func (idx PriceIndex) IsEmpty() bool { return len(idx.slots) == 0 }

// Slots returns a copy of the underlying slot slice.
func (idx PriceIndex) Slots() []PriceSlot {
	if len(idx.slots) == 0 {
		return nil
	}
	cp := make([]PriceSlot, len(idx.slots))
	copy(cp, idx.slots)
	return cp
}

// SlotAt returns the slot that contains t (From ≤ t < Till).
func (idx PriceIndex) SlotAt(t time.Time) (PriceSlot, bool) {
	for _, s := range idx.slots {
		if !t.Before(s.From) && t.Before(s.Till) {
			return s, true
		}
	}
	return PriceSlot{}, false
}

// Level returns the price level for a slot.
func (idx PriceIndex) Level(slot PriceSlot) PriceLevel {
	return DeterminePriceLevel(slot.Price, idx.cheapThreshold, idx.expThreshold)
}

// MedianPrice returns the median of all slot prices in the index.
func (idx PriceIndex) MedianPrice() float64 { return idx.median }

// IsExtreme reports whether a slot is extreme: negative or above spikeMultiplier × median.
func (idx PriceIndex) IsExtreme(slot PriceSlot, spikeMultiplier float64) bool {
	return slot.Price < 0 || slot.Price > idx.MedianPrice()*spikeMultiplier
}

// FindCheapestWindow returns the consecutive block of the requested duration with the lowest
// average price in the [from, deadline) range.
// Slot boundaries determine duration — no fixed slot size is assumed.
func (idx PriceIndex) FindCheapestWindow(duration time.Duration, from, deadline time.Time) ([]PriceSlot, bool) {
	candidates := idx.slotsInRange(from, deadline)
	if len(candidates) == 0 {
		return nil, false
	}

	var best []PriceSlot
	bestAvg := math.MaxFloat64

	for i := range candidates {
		var window []PriceSlot
		var total float64
		for j := i; j < len(candidates); j++ {
			window = append(window, candidates[j])
			total += candidates[j].Price
			if candidates[j].Till.Sub(candidates[i].From) >= duration {
				break
			}
		}
		if len(window) == 0 {
			continue
		}
		if window[len(window)-1].Till.After(deadline) {
			continue
		}
		avg := total / float64(len(window))
		if avg < bestAvg {
			bestAvg = avg
			best = make([]PriceSlot, len(window))
			copy(best, window)
		}
	}

	return best, len(best) > 0
}

// FindCheapestSlots returns the n cheapest non-consecutive slots in [from, deadline).
func (idx PriceIndex) FindCheapestSlots(n int, from, deadline time.Time) []PriceSlot {
	candidates := idx.slotsInRange(from, deadline)
	if len(candidates) == 0 {
		return nil
	}
	sorted := make([]PriceSlot, len(candidates))
	copy(sorted, candidates)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Price < sorted[j].Price
	})
	if n > len(sorted) {
		n = len(sorted)
	}
	return sorted[:n]
}

// HasNegativePrices reports whether any slot in [from, deadline) has a negative price.
func (idx PriceIndex) HasNegativePrices(from, deadline time.Time) bool {
	for _, s := range idx.slotsInRange(from, deadline) {
		if s.Price < 0 {
			return true
		}
	}
	return false
}

// Summary returns a human-readable structure of cheap, expensive, and negative windows.
func (idx PriceIndex) Summary(from, deadline time.Time) IndexSummary {
	summary := IndexSummary{MedianPrice: idx.MedianPrice()}
	candidates := idx.slotsInRange(from, deadline)
	if len(candidates) == 0 {
		return summary
	}

	type run struct {
		level PriceLevel
		neg   bool
		from  time.Time
		till  time.Time
	}

	var runs []run
	for _, s := range candidates {
		level := idx.Level(s)
		neg := s.Price < 0
		if len(runs) > 0 && runs[len(runs)-1].level == level && runs[len(runs)-1].neg == neg {
			runs[len(runs)-1].till = s.Till
		} else {
			runs = append(runs, run{level: level, neg: neg, from: s.From, till: s.Till})
		}
	}

	for _, r := range runs {
		w := SummaryWindow{Level: r.level, From: r.from, Till: r.till}
		switch {
		case r.neg:
			summary.NegativeWindows = append(summary.NegativeWindows, w)
		case r.level == PriceLevelCheap:
			summary.CheapWindows = append(summary.CheapWindows, w)
		case r.level == PriceLevelHigh:
			summary.ExpensiveWindows = append(summary.ExpensiveWindows, w)
			// PriceLevelAverage windows are normal price periods — not worth announcing.
		}
	}

	return summary
}

func (idx PriceIndex) slotsInRange(from, deadline time.Time) []PriceSlot {
	var result []PriceSlot
	for _, s := range idx.slots {
		if !s.Till.After(from) {
			continue
		}
		if !s.From.Before(deadline) {
			break
		}
		result = append(result, s)
	}
	return result
}
