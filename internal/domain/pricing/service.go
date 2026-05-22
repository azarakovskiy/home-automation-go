package pricing

import (
	"fmt"
	"log"
	"sync"
	"time"

	"home-go/internal/tech/homeassistant/entities"
	"home-go/internal/tech/runtime/debug"

	ga "saml.dev/gome-assistant"
)

// Service provides electricity pricing information from Home Assistant.
type Service struct {
	state ga.State

	mu    sync.RWMutex
	index PriceIndex

	now func() time.Time
}

// NewService constructs a pricing service with internal caching.
func NewService(state ga.State) *Service {
	return &Service{
		state: state,
		now:   time.Now,
	}
}

// EventListeners implements component.Component (no custom events needed).
func (s *Service) EventListeners() []ga.EventListener {
	return nil
}

// EntityListeners reacts to HA sensor changes to keep the index fresh.
func (s *Service) EntityListeners() []ga.EntityListener {
	listener := ga.NewEntityListener().
		EntityIds(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
		Call(s.handlePriceSensorChange).
		RunOnStartup().
		Build()
	return []ga.EntityListener{listener}
}

// Schedules implements component.Component (unused).
func (s *Service) Schedules() []ga.DailySchedule { return nil }

// Intervals implements component.Component (unused).
func (s *Service) Intervals() []ga.Interval { return nil }

func (s *Service) handlePriceSensorChange(_ *ga.Service, _ ga.State, data ga.EntityData) {
	debug.Log("Pricing: entity update received for %s", data.TriggerEntityId)
	if err := s.updateFromAttributes(data.ToAttributes); err != nil {
		if fetchErr := s.refreshFromState(); fetchErr != nil {
			log.Printf("ERROR: Failed to refresh prices: %v (fallback error: %v)", err, fetchErr)
		}
	}
}

// UpdateIndex replaces the current PriceIndex with a new one built from slots.
func (s *Service) UpdateIndex(slots []PriceSlot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.index = NewPriceIndex(slots)
}

// CurrentIndex returns the current PriceIndex, or an error if the index is empty.
func (s *Service) CurrentIndex() (PriceIndex, error) {
	s.mu.RLock()
	idx := s.index
	s.mu.RUnlock()
	if idx.IsEmpty() {
		return PriceIndex{}, fmt.Errorf("price index not available")
	}
	return idx, nil
}

// GetPriceSlots returns cached price slots; falls back to direct sensor read when cache empty.
func (s *Service) GetPriceSlots() ([]PriceSlot, error) {
	now := s.now()
	slots := s.getCachedSlots()

	if refresh, reason := s.cacheNeedsRefresh(slots, now); !refresh {
		return slots, nil
	} else {
		log.Printf("Pricing: refreshing cached price slots (%s)", reason)
	}

	if err := s.refreshFromState(); err != nil {
		return nil, err
	}

	slots = s.getCachedSlots()
	if len(slots) == 0 {
		return nil, fmt.Errorf("no price slots available")
	}

	return slots, nil
}

// GetCurrentPrice returns the current electricity price.
func (s *Service) GetCurrentPrice() (float64, error) {
	slots, err := s.GetPriceSlots()
	if err != nil {
		return 0, err
	}

	now := s.now()
	for _, slot := range slots {
		if !now.Before(slot.From) && now.Before(slot.Till) {
			return slot.Price, nil
		}
	}

	return 0, fmt.Errorf("no price slot found for current time")
}

// GetPriceSlotsInWindow returns price slots within a time window.
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

// GetAveragePrice calculates the average price from cached slots.
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

func (s *Service) updateFromAttributes(attrs map[string]any) error {
	if len(attrs) == 0 {
		return fmt.Errorf("entity update missing attributes")
	}
	rawPrices, ok := attrs["prices"]
	if !ok {
		return fmt.Errorf("prices attribute missing")
	}
	return s.applyParsedPrices(rawPrices, "entity event")
}

func (s *Service) refreshFromState() error {
	if s.state == nil {
		return fmt.Errorf("state interface not configured")
	}
	state, err := s.state.Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn)
	if err != nil {
		return fmt.Errorf("failed to get price sensor: %w", err)
	}
	rawPrices, ok := state.Attributes["prices"]
	if !ok {
		return fmt.Errorf("prices attribute not found")
	}
	return s.applyParsedPrices(rawPrices, "HA sensor")
}

func (s *Service) applyParsedPrices(rawPrices any, source string) error {
	slots, err := parsePriceSlots(rawPrices)
	if err != nil {
		return err
	}

	s.mu.RLock()
	current := s.index.Slots()
	s.mu.RUnlock()

	if equalPriceSlots(current, slots) {
		debug.Log("Pricing: index already up to date from %s", source)
		return nil
	}

	s.UpdateIndex(slots)
	log.Printf("Pricing: index updated from %s with %d slots", source, len(slots))
	return nil
}

func (s *Service) getCachedSlots() []PriceSlot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	slots := s.index.slots
	if len(slots) == 0 {
		return nil
	}
	copied := make([]PriceSlot, len(slots))
	copy(copied, slots)
	return copied
}

func (s *Service) cacheNeedsRefresh(slots []PriceSlot, now time.Time) (bool, string) {
	if len(slots) == 0 {
		return true, "cache empty"
	}

	latest := slots[len(slots)-1]
	if !latest.Till.After(now) {
		return true, "latest slot already expired"
	}

	for _, slot := range slots {
		if !now.Before(slot.From) && now.Before(slot.Till) {
			return false, ""
		}
	}

	return true, "current slot not present in cache"
}

func parsePriceSlots(raw any) ([]PriceSlot, error) {
	pricesList, ok := raw.([]any)
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
		priceVal, _ := priceMap["price"].(float64)

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
			Price: priceVal,
		})
	}

	return slots, nil
}

func equalPriceSlots(a, b []PriceSlot) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !a[i].From.Equal(b[i].From) || !a[i].Till.Equal(b[i].Till) || a[i].Price != b[i].Price {
			return false
		}
	}

	return true
}
