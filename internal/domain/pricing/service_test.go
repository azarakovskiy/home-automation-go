package pricing

import (
	"errors"
	"testing"
	"time"

	"home-go/internal/mocks"
	"home-go/internal/tech/homeassistant/entities"

	"go.uber.org/mock/gomock"
	ga "saml.dev/gome-assistant"
)

func TestNewServiceInitializesState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockState := mocks.NewMockState(ctrl)
	service := NewService(mockState)

	if service == nil {
		t.Fatal("NewService returned nil")
	}
	if service.state != mockState {
		t.Fatal("state not set correctly")
	}
}

func TestServiceGetPriceSlotsCachesData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockState := mocks.NewMockState(ctrl)
	mockState.EXPECT().
		Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
		Return(samplePriceState(), nil).
		Times(1)

	service := NewService(mockState)
	service.now = func() time.Time { return time.Date(2025, 10, 23, 10, 30, 0, 0, time.UTC) }

	// First call should hit HA
	slots, err := service.GetPriceSlots()
	if err != nil {
		t.Fatalf("GetPriceSlots returned error: %v", err)
	}
	if len(slots) != 3 {
		t.Fatalf("expected 3 slots, got %d", len(slots))
	}

	// Second call should use cache (no mock expectation configured)
	if _, err := service.GetPriceSlots(); err != nil {
		t.Fatalf("GetPriceSlots second call returned error: %v", err)
	}
}

func TestServiceGetPriceSlotsPropagatesErrors(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockState := mocks.NewMockState(ctrl)
	mockState.EXPECT().
		Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
		Return(ga.EntityState{}, errors.New("sensor not found"))

	service := NewService(mockState)
	if _, err := service.GetPriceSlots(); err == nil {
		t.Fatal("expected error when HA sensor fails")
	}
}

func TestServiceGetPriceSlotsRefreshesStaleCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2025, 10, 23, 10, 30, 0, 0, time.UTC)
	future := now.Add(30 * time.Hour)

	mockState := mocks.NewMockState(ctrl)
	gomock.InOrder(
		mockState.EXPECT().
			Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
			Return(samplePriceState(), nil),
		mockState.EXPECT().
			Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
			Return(samplePriceState(), nil),
	)

	service := NewService(mockState)
	currentTime := now
	service.now = func() time.Time { return currentTime }

	if _, err := service.GetPriceSlots(); err != nil {
		t.Fatalf("first GetPriceSlots failed: %v", err)
	}

	currentTime = future
	if _, err := service.GetPriceSlots(); err != nil {
		t.Fatalf("second GetPriceSlots failed: %v", err)
	}
}

func TestServiceGetCurrentPriceUsesCache(t *testing.T) {
	service := NewService(nil)

	now := time.Date(2025, 10, 23, 10, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	setCachedSlots(service, []PriceSlot{
		{From: now.Add(-30 * time.Minute), Till: now.Add(30 * time.Minute), Price: 0.2},
		{From: now.Add(30 * time.Minute), Till: now.Add(90 * time.Minute), Price: 0.3},
	})

	price, err := service.GetCurrentPrice()
	if err != nil {
		t.Fatalf("GetCurrentPrice returned error: %v", err)
	}
	if price != 0.2 {
		t.Fatalf("expected 0.2, got %.3f", price)
	}
}

func TestServiceGetAveragePrice(t *testing.T) {
	service := NewService(nil)
	now := time.Date(2025, 10, 23, 10, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	setCachedSlots(service, []PriceSlot{
		{From: now.Add(-1 * time.Hour), Till: now, Price: 0.1},
		{From: now, Till: now.Add(1 * time.Hour), Price: 0.3},
	})

	avg, err := service.GetAveragePrice()
	if err != nil {
		t.Fatalf("GetAveragePrice returned error: %v", err)
	}
	if avg != 0.2 {
		t.Fatalf("expected average 0.2, got %.3f", avg)
	}
}

func samplePriceState() ga.EntityState {
	return ga.EntityState{
		Attributes: map[string]any{
			"prices": []any{
				map[string]any{
					"from":  "2025-10-23T10:00:00Z",
					"till":  "2025-10-23T11:00:00Z",
					"price": 0.15,
				},
				map[string]any{
					"from":  "2025-10-23T11:00:00Z",
					"till":  "2025-10-23T12:00:00Z",
					"price": 0.20,
				},
				map[string]any{
					"from":  "2025-10-23T12:00:00Z",
					"till":  "2025-10-23T13:00:00Z",
					"price": 0.25,
				},
			},
		},
	}
}

func setCachedSlots(s *Service, slots []PriceSlot) {
	s.UpdateIndex(slots)
}
