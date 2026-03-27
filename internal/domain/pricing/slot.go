package pricing

import "time"

// PriceSlot represents a time slot with its electricity price.
type PriceSlot struct {
	From  time.Time
	Till  time.Time
	Price float64
}
