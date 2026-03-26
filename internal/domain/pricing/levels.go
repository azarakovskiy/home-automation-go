package pricing

import (
	"math"
	"sort"
)

const (
	PriceBucketSize        = 0.05
	CheapPercentile        = 0.35
	ExpensivePercentile    = 0.7
	MinSamplesForHistogram = 24
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

type Bucket struct {
	Price  float64
	Weight float64
}

func RoundPriceToBucket(price float64) float64 {
	return math.Round(price/PriceBucketSize) * PriceBucketSize
}

func BuildBucketsFromHistogram(hist map[float64]float64) ([]Bucket, float64) {
	buckets := make([]Bucket, 0, len(hist))
	var total float64
	for price, weight := range hist {
		if weight <= 0 {
			continue
		}
		total += weight
		buckets = append(buckets, Bucket{
			Price:  price,
			Weight: weight,
		})
	}

	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Price < buckets[j].Price
	})

	return buckets, total
}

func PercentileFromBuckets(buckets []Bucket, total float64, percentile float64) float64 {
	if len(buckets) == 0 || total == 0 {
		return 0
	}

	target := total * percentile
	cumulative := 0.0
	for _, bucket := range buckets {
		cumulative += bucket.Weight
		if cumulative >= target {
			return bucket.Price
		}
	}

	return buckets[len(buckets)-1].Price
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
