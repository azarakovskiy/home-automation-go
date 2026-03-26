package pricing

import (
	"time"

	domainpricing "home-go/internal/domain/pricing"
)

const (
	minAnnouncementDuration   = 2 * time.Hour
	announcementTimeTolerance = 30 * time.Minute
)

const (
	priceBucketSize        = domainpricing.PriceBucketSize
	cheapPercentile        = domainpricing.CheapPercentile
	expensivePercentile    = domainpricing.ExpensivePercentile
	minSamplesForHistogram = domainpricing.MinSamplesForHistogram
)

type PriceLevel = domainpricing.PriceLevel

const (
	PriceLevelUnknown = domainpricing.PriceLevelUnknown
	PriceLevelCheap   = domainpricing.PriceLevelCheap
	PriceLevelAverage = domainpricing.PriceLevelAverage
	PriceLevelHigh    = domainpricing.PriceLevelHigh
)

type priceWindow struct {
	Level PriceLevel
	Start time.Time
	End   time.Time
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
