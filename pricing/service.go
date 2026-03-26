package pricing

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"home-go/debug"
	"home-go/dryrun"
	"home-go/entities"
	domainpricing "home-go/internal/domain/pricing"
	"home-go/notifications"

	ga "saml.dev/gome-assistant"
)

// PriceSlot is retained here as a compatibility alias for the domain type.
type PriceSlot = domainpricing.PriceSlot

const minAnnouncementInterval = 2 * time.Hour

type notificationSender interface {
	Notify(event notifications.NotificationEvent) error
}

// Service provides electricity pricing information from Home Assistant
type Service struct {
	service *ga.Service
	state   ga.State

	mu                 sync.RWMutex
	priceSlots         []PriceSlot
	histogram          map[float64]float64
	histogramLoaded    bool
	notificationSender notificationSender
	lastAnnouncement   priceWindow
	lastAnnouncementAt time.Time

	now       func() time.Time
	isNightFn func() (bool, error)
	isAwayFn  func() (bool, error)
}

// NewService constructs a pricing service with internal caching and notification support
func NewService(haService *ga.Service, state ga.State) *Service {
	var notifier notificationSender
	if haService != nil {
		notifier = notifications.NewNotificationService(haService)
	}

	s := &Service{
		service:            haService,
		state:              state,
		histogram:          make(map[float64]float64),
		notificationSender: notifier,
		now:                time.Now,
	}

	s.isNightFn = s.defaultIsNightMode
	s.isAwayFn = s.defaultIsAwayMode

	return s
}

// EventListeners implements component.Component (no custom events needed)
func (s *Service) EventListeners() []ga.EventListener {
	return nil
}

// EntityListeners reacts to HA sensor changes so we keep cache + histogram fresh
func (s *Service) EntityListeners() []ga.EntityListener {
	listener := ga.NewEntityListener().
		EntityIds(entities.Sensor.FrankEnergiePricesCurrentElectricityPriceAllIn).
		Call(s.handlePriceSensorChange).
		RunOnStartup().
		Build()

	return []ga.EntityListener{listener}
}

// Schedules implements component.Component (unused)
func (s *Service) Schedules() []ga.DailySchedule {
	return nil
}

// Intervals implements component.Component (unused)
func (s *Service) Intervals() []ga.Interval {
	return nil
}

func (s *Service) handlePriceSensorChange(service *ga.Service, state ga.State, data ga.EntityData) {
	debug.Log("Pricing: entity update received for %s", data.TriggerEntityId)
	if err := s.updateFromAttributes(data.ToAttributes); err != nil {
		// Fall back to a direct fetch if event payload missed attributes
		if fetchErr := s.refreshFromState(); fetchErr != nil {
			log.Printf("ERROR: Failed to refresh prices: %v (fallback error: %v)", err, fetchErr)
		}
	}
}

// GetPriceSlots returns cached price slots; falls back to direct sensor read when cache empty
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

// GetCurrentPrice returns the current electricity price
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

// GetAveragePrice calculates the average price from cached slots
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

// IsCurrentlyExpensive checks if current price is above learned expensive threshold
func (s *Service) IsCurrentlyExpensive() (bool, error) {
	currentPrice, err := s.GetCurrentPrice()
	if err != nil {
		return false, err
	}

	level := s.classifyPrice(currentPrice)
	return level == PriceLevelHigh, nil
}

func (s *Service) updateFromAttributes(attrs map[string]any) error {
	if len(attrs) == 0 {
		return fmt.Errorf("entity update missing attributes")
	}

	rawPrices, ok := attrs["prices"]
	if !ok {
		return fmt.Errorf("prices attribute missing")
	}

	slots, err := parsePriceSlots(rawPrices)
	if err != nil {
		return err
	}

	if !s.updateCache(slots) {
		debug.Log("Pricing: cache already up to date from entity event")
		return nil
	}

	log.Printf("Pricing: cache updated from entity event with %d slots", len(slots))
	s.ingestHistogram(slots)
	s.maybeAnnounce(slots)
	return nil
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

	slots, err := parsePriceSlots(rawPrices)
	if err != nil {
		return err
	}

	if !s.updateCache(slots) {
		debug.Log("Pricing: cache refresh skipped (identical data from HA sensor)")
		return nil
	}

	log.Printf("Pricing: refreshed prices from HA sensor (%d slots)", len(slots))
	s.ingestHistogram(slots)
	s.maybeAnnounce(slots)
	return nil
}

func (s *Service) getCachedSlots() []PriceSlot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.priceSlots) == 0 {
		return nil
	}

	copied := make([]PriceSlot, len(s.priceSlots))
	copy(copied, s.priceSlots)
	return copied
}

func (s *Service) updateCache(slots []PriceSlot) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if equalPriceSlots(s.priceSlots, slots) {
		return false
	}

	s.priceSlots = make([]PriceSlot, len(slots))
	copy(s.priceSlots, slots)
	return true
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

func (s *Service) ingestHistogram(slots []PriceSlot) {
	if len(slots) == 0 {
		return
	}

	if err := s.ensureHistogramLoaded(); err != nil {
		log.Printf("WARNING: Unable to load price histogram: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, slot := range slots {
		bucket := domainpricing.RoundPriceToBucket(slot.Price)
		durationWeight := slot.Till.Sub(slot.From).Hours()
		if durationWeight <= 0 {
			durationWeight = 1
		}
		s.histogram[bucket] += durationWeight
	}

	if err := s.persistHistogramLocked(); err != nil {
		log.Printf("WARNING: Failed to persist price histogram: %v", err)
	}

	debug.Log("Pricing: ingested %d slots into histogram (total buckets=%d)", len(slots), len(s.histogram))
}

func (s *Service) ensureHistogramLoaded() error {
	s.mu.RLock()
	if s.histogramLoaded {
		s.mu.RUnlock()
		return nil
	}
	s.mu.RUnlock()

	if s.state == nil {
		return fmt.Errorf("state interface not configured for histogram")
	}

	state, err := s.state.Get(entities.InputText.EnergyPriceHistogram)
	if err != nil {
		return err
	}

	payload := strings.TrimSpace(state.State)
	if payload == "" {
		s.mu.Lock()
		s.histogramLoaded = true
		s.mu.Unlock()
		debug.Log("Pricing: histogram entity empty, starting fresh")
		return nil
	}

	var stored map[string]float64
	if err := json.Unmarshal([]byte(payload), &stored); err != nil {
		return fmt.Errorf("failed to parse histogram payload: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for key, weight := range stored {
		price, err := strconv.ParseFloat(key, 64)
		if err != nil {
			continue
		}
		s.histogram[price] = weight
	}
	s.histogramLoaded = true
	debug.Log("Pricing: loaded histogram from HA helper with %d buckets", len(s.histogram))

	return nil
}

func (s *Service) persistHistogramLocked() error {
	if s.service == nil || entities.InputText.EnergyPriceHistogram == "" {
		return nil
	}

	payload := make(map[string]float64, len(s.histogram))
	for price, weight := range s.histogram {
		payload[fmt.Sprintf("%.2f", price)] = weight
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode histogram payload: %w", err)
	}

	if err := dryrun.CallWithData(
		"InputText.Set",
		entities.InputText.EnergyPriceHistogram,
		string(data),
		func() error {
			return s.service.InputText.Set(entities.InputText.EnergyPriceHistogram, string(data))
		},
	); err != nil {
		return err
	}

	debug.Log("Pricing: persisted histogram with %d buckets", len(payload))
	return nil
}

func (s *Service) classifyPrice(price float64) PriceLevel {
	hist := s.histogramSnapshot()
	cheap, expensive := thresholdsFromHistogram(hist)

	// Not enough history yet – fall back to the current slot distribution
	if cheap == 0 && expensive == 0 {
		slots := s.getCachedSlots()
		prices := make([]float64, 0, len(slots))
		for _, slot := range slots {
			prices = append(prices, slot.Price)
		}
		cheap, expensive = domainpricing.ComputeThresholdsFromPrices(prices, cheapPercentile, expensivePercentile)
	}

	return domainpricing.DeterminePriceLevel(price, cheap, expensive)
}

func thresholdsFromHistogram(hist map[float64]float64) (float64, float64) {
	buckets, total := domainpricing.BuildBucketsFromHistogram(hist)
	if total < minSamplesForHistogram {
		return 0, 0
	}

	cheap := domainpricing.PercentileFromBuckets(buckets, total, cheapPercentile)
	expensive := domainpricing.PercentileFromBuckets(buckets, total, expensivePercentile)
	return cheap, expensive
}

func (s *Service) histogramSnapshot() map[float64]float64 {
	if err := s.ensureHistogramLoaded(); err != nil {
		log.Printf("WARNING: Unable to ensure histogram: %v", err)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[float64]float64, len(s.histogram))
	for price, weight := range s.histogram {
		snapshot[price] = weight
	}
	return snapshot
}

func (s *Service) maybeAnnounce(slots []PriceSlot) {
	if s.notificationSender == nil || len(slots) == 0 {
		return
	}

	classifier := func(price float64) PriceLevel {
		return s.classifyPrice(price)
	}

	window := buildAnnouncementWindow(slots, s.now(), classifier)
	if window.Level == PriceLevelUnknown {
		return
	}

	if window.Level == PriceLevelAverage {
		return // Nothing actionable to say
	}

	duration := window.End.Sub(window.Start)
	if duration < minAnnouncementDuration {
		return
	}

	if !s.canAnnounce() {
		return
	}

	if s.wasAnnounced(window) {
		return
	}

	hours := int(math.Round(duration.Hours()))
	if hours <= 0 {
		hours = 1
	}
	untilSpeech := notifications.FormatTimeForSpeech(window.End)

	message := fmt.Sprintf("For the next %d hours, electricity prices are %s until %s.",
		hours, window.Level.HumanString(), untilSpeech)

	if err := s.notificationSender.Notify(notifications.NotificationEvent{
		Device:  "pricing",
		Type:    fmt.Sprintf("price_%s_window", window.Level.String()),
		Message: message,
		Data: map[string]any{
			"level":          window.Level.String(),
			"until":          window.End.Format(time.RFC3339),
			"duration_hours": hours,
		},
	}); err != nil {
		log.Printf("WARNING: Failed to send price window notification: %v", err)
		return
	}

	log.Printf("Pricing: announcing %s prices for next %d hours (until %s)", window.Level.String(), hours, untilSpeech)
	s.recordAnnouncement(window)
}

func (s *Service) canAnnounce() bool {
	s.mu.RLock()
	lastAt := s.lastAnnouncementAt
	s.mu.RUnlock()

	if !lastAt.IsZero() && s.now().Sub(lastAt) < minAnnouncementInterval {
		return false
	}

	if s.isNightFn != nil {
		isNight, err := s.isNightFn()
		if err != nil {
			log.Printf("WARNING: Failed to detect night mode: %v", err)
		} else if isNight {
			return false
		}
	}

	if s.isAwayFn != nil {
		isAway, err := s.isAwayFn()
		if err != nil {
			log.Printf("WARNING: Failed to read house mode: %v", err)
		} else if isAway {
			return false
		}
	}

	return true
}

func (s *Service) wasAnnounced(window priceWindow) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	last := s.lastAnnouncement
	if last.Level != window.Level || last.Level == PriceLevelUnknown {
		return false
	}

	if math.Abs(last.Start.Sub(window.Start).Minutes()) > announcementTimeTolerance.Minutes() {
		return false
	}

	if math.Abs(last.End.Sub(window.End).Minutes()) > announcementTimeTolerance.Minutes() {
		return false
	}

	return true
}

func (s *Service) recordAnnouncement(window priceWindow) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastAnnouncement = window
	s.lastAnnouncementAt = s.now()
}

func (s *Service) defaultIsNightMode() (bool, error) {
	if s.state == nil {
		return false, fmt.Errorf("state interface not configured")
	}

	state, err := s.state.Get(entities.InputSelect.DaytimeMode)
	if err != nil {
		return false, fmt.Errorf("failed to get daytime mode: %w", err)
	}

	return state.State == "Night", nil
}

func (s *Service) defaultIsAwayMode() (bool, error) {
	if s.state == nil {
		return false, fmt.Errorf("state interface not configured")
	}

	state, err := s.state.Get(entities.InputSelect.HouseMode)
	if err != nil {
		return false, fmt.Errorf("failed to get house mode: %w", err)
	}

	mode := state.State
	return mode == "Away" || mode == "Travel", nil
}
