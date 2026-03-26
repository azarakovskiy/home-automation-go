package pricing

import (
	"errors"
	"strings"
	"testing"
	"time"

	"home-go/internal/tech/homeassistant/entities"
	"home-go/internal/mocks"
	"home-go/internal/tech/homeassistant/notifications"

	"go.uber.org/mock/gomock"
	ga "saml.dev/gome-assistant"
)

func TestNewServiceInitializesState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockState := mocks.NewMockStateInterface(ctrl)
	service := NewService(nil, mockState)

	if service == nil {
		t.Fatal("NewService returned nil")
	}
	if service.state != mockState {
		t.Fatal("state not set correctly")
	}
	if service.service != nil {
		t.Fatal("service should be nil when GA service not provided")
	}
	if service.notificationSender != nil {
		t.Fatal("notification sender should be nil when no GA service provided")
	}
}

func TestServiceGetPriceSlotsCachesData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockState := mocks.NewMockStateInterface(ctrl)
	mockState.EXPECT().
		Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
		Return(samplePriceState(), nil).
		Times(1)

	service := NewService(nil, mockState)
	service.now = func() time.Time { return time.Date(2025, 10, 23, 10, 30, 0, 0, time.UTC) }
	service.histogramLoaded = true

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

	mockState := mocks.NewMockStateInterface(ctrl)
	mockState.EXPECT().
		Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
		Return(ga.EntityState{}, errors.New("sensor not found"))

	service := NewService(nil, mockState)
	if _, err := service.GetPriceSlots(); err == nil {
		t.Fatal("expected error when HA sensor fails")
	}
}

func TestServiceGetPriceSlotsRefreshesStaleCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Date(2025, 10, 23, 10, 30, 0, 0, time.UTC)
	future := now.Add(30 * time.Hour)

	firstState := samplePriceState()
	secondState := samplePriceState()

	mockState := mocks.NewMockStateInterface(ctrl)
	gomock.InOrder(
		mockState.EXPECT().
			Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
			Return(firstState, nil),
		mockState.EXPECT().
			Get(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
			Return(secondState, nil),
	)

	service := NewService(nil, mockState)
	currentTime := now
	service.now = func() time.Time { return currentTime }
	service.histogramLoaded = true

	if _, err := service.GetPriceSlots(); err != nil {
		t.Fatalf("first GetPriceSlots failed: %v", err)
	}

	currentTime = future
	if _, err := service.GetPriceSlots(); err != nil {
		t.Fatalf("second GetPriceSlots failed: %v", err)
	}
}

func TestServiceGetCurrentPriceUsesCache(t *testing.T) {
	service := NewService(nil, nil)

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
	service := NewService(nil, nil)
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

func TestThresholdsFromHistogram(t *testing.T) {
	hist := map[float64]float64{
		0.10: 20,
		0.20: 10,
		0.40: 5,
	}

	cheap, expensive := thresholdsFromHistogram(hist)
	if cheap != 0.10 {
		t.Fatalf("expected cheap threshold 0.10, got %.2f", cheap)
	}
	if expensive != 0.20 {
		t.Fatalf("expected expensive threshold 0.20, got %.2f", expensive)
	}
}

func TestBuildAnnouncementWindow(t *testing.T) {
	now := time.Date(2025, 10, 23, 10, 0, 0, 0, time.UTC)
	slots := []PriceSlot{
		{From: now.Add(-1 * time.Hour), Till: now.Add(-30 * time.Minute), Price: 0.3},
		{From: now, Till: now.Add(1 * time.Hour), Price: 0.1},
		{From: now.Add(1 * time.Hour), Till: now.Add(2 * time.Hour), Price: 0.1},
		{From: now.Add(2 * time.Hour), Till: now.Add(3 * time.Hour), Price: 0.3},
	}

	classifier := func(price float64) PriceLevel {
		if price <= 0.1 {
			return PriceLevelCheap
		}
		return PriceLevelHigh
	}

	window := buildAnnouncementWindow(slots, now, classifier)
	if window.Level != PriceLevelCheap {
		t.Fatalf("expected cheap level, got %s", window.Level.String())
	}
	if window.End.Sub(window.Start) != 2*time.Hour {
		t.Fatalf("expected 2h window, got %s", window.End.Sub(window.Start))
	}
}

func TestMaybeAnnounceSendsSingleNotification(t *testing.T) {
	now := time.Date(2025, 10, 23, 10, 0, 0, 0, time.UTC)
	service := NewService(nil, nil)
	service.now = func() time.Time { return now }

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockNotifier := mocks.NewMockNotificationSenderInterface(ctrl)
	var events []notifications.NotificationEvent
	mockNotifier.EXPECT().
		Notify(gomock.Any()).
		Do(func(event notifications.NotificationEvent) {
			events = append(events, event)
		}).
		Times(1)
	service.notificationSender = mockNotifier
	service.isNightFn = func() (bool, error) { return false, nil }
	service.isAwayFn = func() (bool, error) { return false, nil }
	service.histogram = map[float64]float64{
		0.15: 40,
		0.30: 30,
	}
	service.histogramLoaded = true

	slots := []PriceSlot{
		{From: now, Till: now.Add(time.Hour), Price: 0.15},
		{From: now.Add(time.Hour), Till: now.Add(2 * time.Hour), Price: 0.15},
		{From: now.Add(2 * time.Hour), Till: now.Add(3 * time.Hour), Price: 0.3},
	}

	service.maybeAnnounce(slots)

	if len(events) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(events))
	}
	if !strings.Contains(events[0].Message, "cheap") {
		t.Fatalf("expected message to mention cheap prices, got: %s", events[0].Message)
	}

	// Calling again with same data should not send duplicate
	service.maybeAnnounce(slots)
	if len(events) != 1 {
		t.Fatalf("expected no duplicate notifications, got %d", len(events))
	}
}

func TestMaybeAnnounceSkippedWhenNight(t *testing.T) {
	now := time.Date(2025, 10, 23, 22, 0, 0, 0, time.UTC)
	service := NewService(nil, nil)
	service.now = func() time.Time { return now }

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	service.notificationSender = mocks.NewMockNotificationSenderInterface(ctrl)
	service.isNightFn = func() (bool, error) { return true, nil }
	service.isAwayFn = func() (bool, error) { return false, nil }
	service.histogram = map[float64]float64{
		0.15: 40,
		0.30: 30,
	}
	service.histogramLoaded = true

	slots := []PriceSlot{
		{From: now, Till: now.Add(3 * time.Hour), Price: 0.15},
	}

	service.maybeAnnounce(slots)
}

func TestMaybeAnnounceRespectsMinInterval(t *testing.T) {
	base := time.Date(2025, 10, 23, 8, 0, 0, 0, time.UTC)
	current := base

	service := NewService(nil, nil)
	service.now = func() time.Time { return current }

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockNotifier := mocks.NewMockNotificationSenderInterface(ctrl)
	var events []notifications.NotificationEvent
	mockNotifier.EXPECT().
		Notify(gomock.Any()).
		Do(func(event notifications.NotificationEvent) {
			events = append(events, event)
		}).
		Times(2)
	service.notificationSender = mockNotifier
	service.isNightFn = func() (bool, error) { return false, nil }
	service.isAwayFn = func() (bool, error) { return false, nil }
	service.histogram = map[float64]float64{
		0.15: 40,
		0.35: 40,
	}
	service.histogramLoaded = true

	cheapSlots := []PriceSlot{
		{From: base, Till: base.Add(time.Hour), Price: 0.15},
		{From: base.Add(time.Hour), Till: base.Add(2 * time.Hour), Price: 0.15},
	}

	expensiveSlots := []PriceSlot{
		{From: base.Add(2 * time.Hour), Till: base.Add(3 * time.Hour), Price: 0.35},
		{From: base.Add(3 * time.Hour), Till: base.Add(4 * time.Hour), Price: 0.35},
	}

	service.maybeAnnounce(cheapSlots)
	if len(events) != 1 {
		t.Fatalf("expected initial notification, got %d", len(events))
	}

	current = current.Add(time.Hour)
	service.maybeAnnounce(expensiveSlots)
	if len(events) != 1 {
		t.Fatalf("expected throttling within 2h window, got %d notifications", len(events))
	}

	current = current.Add(time.Hour)
	service.maybeAnnounce(expensiveSlots)
	if len(events) != 2 {
		t.Fatalf("expected notification after 2h window, got %d", len(events))
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.priceSlots = make([]PriceSlot, len(slots))
	copy(s.priceSlots, slots)
}
