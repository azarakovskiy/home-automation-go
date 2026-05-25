package pricing

import (
	"math"
	"sort"
)

const (
	CheapPercentile     = 0.35
	ExpensivePercentile = 0.7
)

// PriceLevel represents how favorable the current price is.
type PriceLevel int

const (
	PriceLevelUnknown PriceLevel = iota
	PriceLevelCheap
	PriceLevelAverage
	PriceLevelHigh
)

func (l PriceLevel) String() string {
	switch l {
	case PriceLevelCheap:
		return "cheap"
	case PriceLevelAverage:
		return "average"
	case PriceLevelHigh:
		return "expensive"
	default:
		return "unknown"
	}
}

// HumanString returns a friendly adjective for notifications.
func (l PriceLevel) HumanString() string {
	switch l {
	case PriceLevelCheap:
		return "cheap"
	case PriceLevelHigh:
		return "expensive"
	case PriceLevelAverage:
		return "moderate"
	default:
		return "unknown"
	}
}

func DeterminePriceLevel(price float64, cheapThreshold, expensiveThreshold float64) PriceLevel {
	if cheapThreshold == 0 && expensiveThreshold == 0 {
		return PriceLevelUnknown
	}

	switch {
	case price <= cheapThreshold:
		return PriceLevelCheap
	case price >= expensiveThreshold:
		return PriceLevelHigh
	default:
		return PriceLevelAverage
	}
}

func ComputeThresholdsFromPrices(prices []float64, cheapPct, expensivePct float64) (float64, float64) {
	if len(prices) == 0 {
		return 0, 0
	}

	sorted := make([]float64, len(prices))
	copy(sorted, prices)
	sort.Float64s(sorted)

	cheapIdx := int(math.Max(0, math.Min(float64(len(sorted)-1), math.Round(float64(len(sorted)-1)*cheapPct))))
	expensiveIdx := int(math.Max(0, math.Min(float64(len(sorted)-1), math.Round(float64(len(sorted)-1)*expensivePct))))

	return sorted[cheapIdx], sorted[expensiveIdx]
}
