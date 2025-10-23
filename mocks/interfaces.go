package mocks

import (
	"time"

	ga "saml.dev/gome-assistant"
)

//go:generate mockgen -destination=mock_ha_service.go -package=mocks -source=interfaces.go

// HAService wraps the gome-assistant Service for mocking
type HAService interface {
	// Add methods as needed for testing
}

// HAState wraps the gome-assistant State for mocking
type HAState interface {
	Get(entityID string) (ga.EntityState, error)
}

// PriceSlot represents a time slot with its electricity price
type PriceSlot struct {
	From  time.Time
	Till  time.Time
	Price float64
}

// PricingService interface for mocking pricing.Service
type PricingService interface {
	GetPriceSlots() ([]PriceSlot, error)
	GetCurrentPrice() (float64, error)
	GetPriceSlotsInWindow(from, until time.Time) ([]PriceSlot, error)
	GetAveragePrice() (float64, error)
	IsCurrentlyExpensive() (bool, error)
}

