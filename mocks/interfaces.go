package mocks

import (
	"time"

	ga "saml.dev/gome-assistant"
)

//go:generate mockgen -destination=mock_interfaces.go -package=mocks -source=interfaces.go

// StateInterface wraps ga.State for testing
type StateInterface interface {
	Get(entityID string) (ga.EntityState, error)
}

// SwitchInterface wraps the Switch service for testing
type SwitchInterface interface {
	TurnOn(entityID string) error
	TurnOff(entityID string) error
}

// InputBooleanInterface wraps the InputBoolean service for testing
type InputBooleanInterface interface {
	TurnOn(entityID string) error
	TurnOff(entityID string) error
	Toggle(entityID string) error
}

// InputNumberInterface wraps the InputNumber service for testing
type InputNumberInterface interface {
	Set(entityID string, value float32) error
}

// InputTextInterface wraps the InputText service for testing
type InputTextInterface interface {
	Set(entityID string, value string) error
}

// InputDatetimeInterface wraps the InputDatetime service for testing
type InputDatetimeInterface interface {
	Set(entityID string, value time.Time) error
}

// HomeAssistantInterface wraps the HomeAssistant service for testing
type HomeAssistantInterface interface {
	TurnOn(entityID string, serviceData ...map[string]any) error
	TurnOff(entityID string) error
	Toggle(entityID string, serviceData ...map[string]any) error
}

// PricingServiceInterface for mocking our pricing.Service
type PricingServiceInterface interface {
	GetPriceSlots() ([]PriceSlot, error)
	GetCurrentPrice() (float64, error)
	GetPriceSlotsInWindow(from, until time.Time) ([]PriceSlot, error)
	GetAveragePrice() (float64, error)
	IsCurrentlyExpensive() (bool, error)
}

// PriceSlot represents a time slot with its electricity price
type PriceSlot struct {
	From  time.Time
	Till  time.Time
	Price float64
}

