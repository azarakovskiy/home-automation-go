package priceannouncer

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"home-go/internal/domain/pricing"
	"home-go/internal/tech/homeassistant/notifications"

	ga "saml.dev/gome-assistant"
)

type fakeStore struct {
	last     time.Time
	writeErr error // returned by SetLastAnnouncedDate; does not update last when set
}

func (f *fakeStore) LastAnnouncedDate(_ context.Context) (time.Time, error) {
	return f.last, nil
}
func (f *fakeStore) SetLastAnnouncedDate(_ context.Context, t time.Time) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.last = t
	return nil
}

type fakeSender struct {
	events []notifications.Event
}

func (f *fakeSender) Notify(e notifications.Event) error {
	f.events = append(f.events, e)
	return nil
}

type fakeModes struct {
	night bool
	away  bool
}

func (f *fakeModes) IsNight() (bool, error) { return f.night, nil }
func (f *fakeModes) IsAway() (bool, error)  { return f.away, nil }

func makePricingService(slots []pricing.PriceSlot) *pricing.Service {
	svc := pricing.NewService(nil)
	svc.UpdateIndex(slots)
	return svc
}

func baseSlots(base time.Time) []pricing.PriceSlot {
	prices := []float64{0.10, 0.12, 0.20, 0.30, 0.28, 0.15, 0.11, 0.10}
	slots := make([]pricing.PriceSlot, len(prices))
	for i, p := range prices {
		slots[i] = pricing.PriceSlot{
			From:  base.Add(time.Duration(i) * time.Hour),
			Till:  base.Add(time.Duration(i+1) * time.Hour),
			Price: p,
		}
	}
	return slots
}

func TestAnnouncer_MorningSummary_SendsOncePerDay(t *testing.T) {
	base := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	store := &fakeStore{}
	sender := &fakeSender{}
	modes := &fakeModes{}
	svc := makePricingService(baseSlots(base))

	a := New(svc, modes, sender, store, AnnouncerConfig{
		SpikeMultiplier:    3.0,
		MinExtremeDuration: time.Hour,
	})
	a.now = func() time.Time { return base }

	a.HandleMorning(nil, nil, ga.EntityData{})

	if len(sender.events) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(sender.events))
	}

	a.HandleMorning(nil, nil, ga.EntityData{})
	if len(sender.events) != 1 {
		t.Fatalf("expected no duplicate on same day, got %d", len(sender.events))
	}
}

func TestAnnouncer_MorningSummary_SuppressedAtNight(t *testing.T) {
	base := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	store := &fakeStore{}
	sender := &fakeSender{}
	modes := &fakeModes{night: true}
	svc := makePricingService(baseSlots(base))

	a := New(svc, modes, sender, store, AnnouncerConfig{
		SpikeMultiplier:    3.0,
		MinExtremeDuration: time.Hour,
	})
	a.now = func() time.Time { return base }

	a.HandleMorning(nil, nil, ga.EntityData{})

	if len(sender.events) != 0 {
		t.Fatalf("expected no notification during night, got %d", len(sender.events))
	}
}

func TestAnnouncer_OnDemand_NoCooldown(t *testing.T) {
	base := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	store := &fakeStore{}
	sender := &fakeSender{}
	modes := &fakeModes{}
	svc := makePricingService(baseSlots(base))

	a := New(svc, modes, sender, store, AnnouncerConfig{
		SpikeMultiplier:    3.0,
		MinExtremeDuration: time.Hour,
	})
	a.now = func() time.Time { return base }

	a.HandleOnDemand()
	a.HandleOnDemand()

	if len(sender.events) != 2 {
		t.Fatalf("expected 2 on-demand notifications, got %d", len(sender.events))
	}
}

func TestAnnouncer_Reactive_FiresOnExtremeRun(t *testing.T) {
	base := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	prices := []float64{0.10, 0.12, 0.40, 1.50, 1.40, 1.35, 0.15}
	slots := make([]pricing.PriceSlot, len(prices))
	for i, p := range prices {
		slots[i] = pricing.PriceSlot{
			From:  base.Add(time.Duration(i) * time.Hour),
			Till:  base.Add(time.Duration(i+1) * time.Hour),
			Price: p,
		}
	}

	store := &fakeStore{}
	sender := &fakeSender{}
	modes := &fakeModes{}
	svc := makePricingService(slots)

	a := New(svc, modes, sender, store, AnnouncerConfig{
		SpikeMultiplier:    3.0,
		MinExtremeDuration: time.Hour,
	})
	a.now = func() time.Time { return base }

	a.handlePriceUpdate(nil, nil, ga.EntityData{})

	if len(sender.events) == 0 {
		t.Fatal("expected reactive alert for 3-hour spike run")
	}
}

func TestAnnouncer_Reactive_IgnoresShortExtremeRun(t *testing.T) {
	base := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	prices := []float64{0.10, 0.12, 1.50, 0.15, 0.12}
	slots := make([]pricing.PriceSlot, len(prices))
	for i, p := range prices {
		slots[i] = pricing.PriceSlot{
			From:  base.Add(time.Duration(i) * time.Hour),
			Till:  base.Add(time.Duration(i+1) * time.Hour),
			Price: p,
		}
	}

	store := &fakeStore{}
	sender := &fakeSender{}
	modes := &fakeModes{}
	svc := makePricingService(slots)

	a := New(svc, modes, sender, store, AnnouncerConfig{
		SpikeMultiplier:    3.0,
		MinExtremeDuration: 2 * time.Hour,
	})
	a.now = func() time.Time { return base }

	a.handlePriceUpdate(nil, nil, ga.EntityData{})

	if len(sender.events) != 0 {
		t.Fatalf("expected no alert for sub-threshold spike run, got %d", len(sender.events))
	}
}

func TestAnnouncer_MorningSummary_SuppressedWhenAway(t *testing.T) {
	base := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	store := &fakeStore{}
	sender := &fakeSender{}
	modes := &fakeModes{away: true}
	svc := makePricingService(baseSlots(base))

	a := New(svc, modes, sender, store, AnnouncerConfig{
		SpikeMultiplier:    3.0,
		MinExtremeDuration: time.Hour,
	})
	a.now = func() time.Time { return base }

	a.HandleMorning(nil, nil, ga.EntityData{})

	if len(sender.events) != 0 {
		t.Fatalf("expected no notification when away, got %d", len(sender.events))
	}
}

func TestAnnouncer_Reactive_IndexNotReady(t *testing.T) {
	store := &fakeStore{}
	sender := &fakeSender{}
	modes := &fakeModes{}
	svc := pricing.NewService(nil) // no UpdateIndex — CurrentIndex() returns error

	a := New(svc, modes, sender, store, AnnouncerConfig{
		SpikeMultiplier:    3.0,
		MinExtremeDuration: time.Hour,
	})

	a.handlePriceUpdate(nil, nil, ga.EntityData{})

	if len(sender.events) != 0 {
		t.Fatalf("expected no notification when index not ready, got %d", len(sender.events))
	}
}

func TestAnnouncer_MorningSummary_NoSendWhenPersistFails(t *testing.T) {
	base := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	store := &fakeStore{writeErr: fmt.Errorf("db down")}
	sender := &fakeSender{}
	modes := &fakeModes{}
	svc := makePricingService(baseSlots(base))

	a := New(svc, modes, sender, store, AnnouncerConfig{
		SpikeMultiplier:    3.0,
		MinExtremeDuration: time.Hour,
	})
	a.now = func() time.Time { return base }

	a.HandleMorning(nil, nil, ga.EntityData{})

	if len(sender.events) != 0 {
		t.Fatalf("expected no notification when state persist fails, got %d", len(sender.events))
	}
}

func TestAnnouncer_OnDemand_MorningFormat(t *testing.T) {
	// Hour 8 < 11 → morning bucket; slots have a cheap window.
	base := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	// prices: three very cheap slots at hours 8-11, rest average
	prices := []float64{0.01, 0.01, 0.01, 0.20, 0.20, 0.20, 0.20, 0.20}
	slots := make([]pricing.PriceSlot, len(prices))
	for i, p := range prices {
		slots[i] = pricing.PriceSlot{
			From:  base.Add(time.Duration(i) * time.Hour),
			Till:  base.Add(time.Duration(i+1) * time.Hour),
			Price: p,
		}
	}
	sender := &fakeSender{}
	modes := &fakeModes{}
	svc := makePricingService(slots)

	a := New(svc, modes, sender, nil, AnnouncerConfig{
		SpikeMultiplier:    3.0,
		MinExtremeDuration: time.Hour,
	})
	a.now = func() time.Time { return base }

	a.HandleOnDemand()

	if len(sender.events) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(sender.events))
	}
	if !strings.Contains(strings.ToLower(sender.events[0].Message), "cheap") {
		t.Errorf("morning message with cheap window should mention cheap; got: %s", sender.events[0].Message)
	}
}

func TestAnnouncer_OnDemand_AfternoonFormat(t *testing.T) {
	// Slots: hours 8-16. Cheap at hours 12-15 (indices 4-6), rest average.
	// now = 12:00, hour 12 >= 11 → afternoon bucket; current slot is cheap.
	base := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	prices := []float64{0.20, 0.20, 0.20, 0.20, 0.01, 0.01, 0.01, 0.20}
	slots := make([]pricing.PriceSlot, len(prices))
	for i, p := range prices {
		slots[i] = pricing.PriceSlot{
			From:  base.Add(time.Duration(i) * time.Hour),
			Till:  base.Add(time.Duration(i+1) * time.Hour),
			Price: p,
		}
	}
	sender := &fakeSender{}
	modes := &fakeModes{}
	svc := makePricingService(slots)

	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC) // in cheap window, hour 12
	a := New(svc, modes, sender, nil, AnnouncerConfig{
		SpikeMultiplier:    3.0,
		MinExtremeDuration: time.Hour,
	})
	a.now = func() time.Time { return now }

	a.HandleOnDemand()

	if len(sender.events) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(sender.events))
	}
	if !strings.Contains(strings.ToLower(sender.events[0].Message), "cheap") {
		t.Errorf("afternoon message when current is cheap should mention cheap; got: %s", sender.events[0].Message)
	}
}

func TestAnnouncer_Reactive_NoDuplicateAlert(t *testing.T) {
	base := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	// Same spike setup as TestAnnouncer_Reactive_FiresOnExtremeRun.
	prices := []float64{0.10, 0.12, 0.40, 1.50, 1.40, 1.35, 0.15}
	slots := make([]pricing.PriceSlot, len(prices))
	for i, p := range prices {
		slots[i] = pricing.PriceSlot{
			From:  base.Add(time.Duration(i) * time.Hour),
			Till:  base.Add(time.Duration(i+1) * time.Hour),
			Price: p,
		}
	}
	sender := &fakeSender{}
	modes := &fakeModes{}
	svc := makePricingService(slots)

	a := New(svc, modes, sender, nil, AnnouncerConfig{
		SpikeMultiplier:    3.0,
		MinExtremeDuration: time.Hour,
	})
	a.now = func() time.Time { return base }

	a.handlePriceUpdate(nil, nil, ga.EntityData{})
	a.handlePriceUpdate(nil, nil, ga.EntityData{})

	if len(sender.events) != 1 {
		t.Fatalf("expected 1 alert (no duplicate for same run), got %d", len(sender.events))
	}
}
