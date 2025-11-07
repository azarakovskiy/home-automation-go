package pricing

import (
	"math"
	"sort"
	"time"
)

const (
	priceBucketSize           = 0.05
	cheapPercentile           = 0.35
	expensivePercentile       = 0.7
	minSamplesForHistogram    = 24
	minAnnouncementDuration   = 2 * time.Hour
	announcementTimeTolerance = 30 * time.Minute
)

// PriceLevel represents how favorable the current price is
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

// HumanString returns a friendly adjective for notifications
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

type priceBucket struct {
	price  float64
	weight float64
}

type priceWindow struct {
	Level PriceLevel
	Start time.Time
	End   time.Time
}

func roundPriceToBucket(price float64) float64 {
	return math.Round(price/priceBucketSize) * priceBucketSize
}

func buildBucketsFromHistogram(hist map[float64]float64) ([]priceBucket, float64) {
	buckets := make([]priceBucket, 0, len(hist))
	var total float64
	for price, weight := range hist {
		if weight <= 0 {
			continue
		}
		total += weight
		buckets = append(buckets, priceBucket{
			price:  price,
			weight: weight,
		})
	}

	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].price < buckets[j].price
	})

	return buckets, total
}

func percentileFromBuckets(buckets []priceBucket, total float64, percentile float64) float64 {
	if len(buckets) == 0 || total == 0 {
		return 0
	}

	target := total * percentile
	cumulative := 0.0
	for _, bucket := range buckets {
		cumulative += bucket.weight
		if cumulative >= target {
			return bucket.price
		}
	}

	return buckets[len(buckets)-1].price
}

func determinePriceLevel(price float64, cheapThreshold, expensiveThreshold float64) PriceLevel {
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

func computeThresholdsFromPrices(prices []float64, cheapPct, expensivePct float64) (float64, float64) {
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

func buildAnnouncementWindow(slots []PriceSlot, now time.Time, classify func(float64) PriceLevel) priceWindow {
	for i := range slots {
		slot := slots[i]
		if slot.Till.Before(now) {
			continue
		}

		level := classify(slot.Price)
		if level == PriceLevelUnknown {
			continue
		}

		start := slot.From
		if now.After(start) {
			start = now
		}
		end := slot.Till

		for j := i + 1; j < len(slots); j++ {
			next := slots[j]
			if next.From.After(end) && next.From.Sub(end) > time.Minute {
				break
			}
			nextLevel := classify(next.Price)
			if nextLevel != level {
				break
			}
			end = next.Till
		}

		return priceWindow{
			Level: level,
			Start: start,
			End:   end,
		}
	}

	return priceWindow{Level: PriceLevelUnknown}
}
