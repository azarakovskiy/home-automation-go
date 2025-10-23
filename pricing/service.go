package pricing

import (
	"fmt"
	"time"

	"home-go/entities"

	ga "saml.dev/gome-assistant"
)

// PriceSlot represents a time slot with its electricity price
type PriceSlot struct {
	From  time.Time
	Till  time.Time
	Price float64 // Price in EUR/kWh
}

// Service provides electricity pricing information from Home Assistant
type Service struct {
	state ga.State
}

func NewService(state ga.State) *Service {
	return &Service{
		state: state,
	}
}

// GetPriceSlots parses the prices attribute from HA sensor
// Returns price slots with from/till timestamps and prices
func (s *Service) GetPriceSlots() ([]PriceSlot, error) {
	state, err := s.state.Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn)
	if err != nil {
		return nil, fmt.Errorf("failed to get price sensor: %w", err)
	}

	// Parse the "prices" attribute
	pricesAttr, ok := state.Attributes["prices"]
	if !ok {
		return nil, fmt.Errorf("prices attribute not found")
	}

	// Type assertion to slice of maps
	pricesList, ok := pricesAttr.([]any)
	if !ok {
		return nil, fmt.Errorf("prices attribute is not a list")
	}

	var slots []PriceSlot
	for _, item := range pricesList {
		priceMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		fromStr, _ := priceMap["from"].(string)
		tillStr, _ := priceMap["till"].(string)
		price, _ := priceMap["price"].(float64)

		from, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			continue
		}

		till, err := time.Parse(time.RFC3339, tillStr)
		if err != nil {
			continue
		}

		slots = append(slots, PriceSlot{
			From:  from,
			Till:  till,
			Price: price,
		})
	}

	return slots, nil
}

// GetCurrentPrice returns the current electricity price
func (s *Service) GetCurrentPrice() (float64, error) {
	slots, err := s.GetPriceSlots()
	if err != nil {
		return 0, err
	}

	now := time.Now()
	for _, slot := range slots {
		if now.After(slot.From) && now.Before(slot.Till) {
			return slot.Price, nil
		}
	}

	return 0, fmt.Errorf("no price slot found for current time")
}

// GetPriceSlotsInWindow returns price slots within a time window
func (s *Service) GetPriceSlotsInWindow(from, until time.Time) ([]PriceSlot, error) {
	allSlots, err := s.GetPriceSlots()
	if err != nil {
		return nil, err
	}

	var result []PriceSlot
	for _, slot := range allSlots {
		if slot.From.After(from) && slot.From.Before(until) {
			result = append(result, slot)
		}
	}

	return result, nil
}

// GetAveragePrice calculates the average price from all available price slots
func (s *Service) GetAveragePrice() (float64, error) {
	slots, err := s.GetPriceSlots()
	if err != nil {
		return 0, err
	}

	if len(slots) == 0 {
		return 0, fmt.Errorf("no price slots available")
	}

	var total float64
	for _, slot := range slots {
		total += slot.Price
	}

	return total / float64(len(slots)), nil
}

// IsCurrentlyExpensive checks if current price is above the average of all available prices
func (s *Service) IsCurrentlyExpensive() (bool, error) {
	currentPrice, err := s.GetCurrentPrice()
	if err != nil {
		return false, err
	}

	avgPrice, err := s.GetAveragePrice()
	if err != nil {
		return false, err
	}

	return currentPrice > avgPrice, nil
}
