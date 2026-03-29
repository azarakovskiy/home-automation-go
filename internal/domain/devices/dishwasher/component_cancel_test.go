package dishwasher_test

import (
	"strings"
	"testing"
	"time"

	dishwasher "home-go/internal/domain/devices/dishwasher"
)

func TestCancelPendingScheduleFromDashboardCancelsPendingSchedule(t *testing.T) {
	store := &dishwasher.TestScheduleStore{}
	d := dishwasher.NewTestDishwasher(store)
	notifier := dishwasher.NewTestNotificationService()
	d.SetNotificationSenderForTest(notifier)
	startTime := time.Date(2025, time.October, 10, 21, 0, 0, 0, time.UTC)
	d.SetPendingScheduleForTest(&dishwasher.PendingSchedule{
		StartTime: startTime,
	})

	d.CancelPendingScheduleFromDashboardForTest()

	if d.PendingScheduleForTest() != nil {
		t.Fatal("expected pending schedule to be cleared")
	}
	if store.ClearCalls != 1 {
		t.Fatalf("ClearCalls = %d, want 1", store.ClearCalls)
	}

	event, ok := notifier.LastEvent()
	if !ok {
		t.Fatal("expected cancellation announcement")
	}
	if event.Type != "cancelled" {
		t.Fatalf("expected cancelled notification, got %s", event.Type)
	}
	if !strings.Contains(event.Message, "Dishwasher schedule for") {
		t.Fatalf("unexpected message: %s", event.Message)
	}
	if got := event.Data["reason"]; got != "dashboard switch turned off" {
		t.Fatalf("expected reason metadata, got %v", got)
	}
	if got := event.Data["start_time_text"]; got != "9 PM" {
		t.Fatalf("unexpected start time text: %v", got)
	}
}

func TestHandleScheduleRequestCancel(t *testing.T) {
	store := &dishwasher.TestScheduleStore{}
	d := dishwasher.NewTestDishwasher(store)
	notifier := dishwasher.NewTestNotificationService()
	d.SetNotificationSenderForTest(notifier)
	startTime := time.Date(2025, time.October, 11, 15, 0, 0, 0, time.UTC)
	d.SetPendingScheduleForTest(&dishwasher.PendingSchedule{
		StartTime: startTime,
	})

	d.HandleScheduleRequestForTest(dishwasher.ScheduleRequest{
		Device: "dishwasher",
		Mode:   string(dishwasher.ModeCancel),
	})

	if d.PendingScheduleForTest() != nil {
		t.Fatal("expected pending schedule to be cleared by cancel request")
	}
	if store.ClearCalls != 1 {
		t.Fatalf("ClearCalls = %d, want 1", store.ClearCalls)
	}

	event, ok := notifier.LastEvent()
	if !ok {
		t.Fatal("expected cancellation announcement")
	}
	if event.Data["reason"] != "cancel event" {
		t.Fatalf("expected cancel event reason, got %v", event.Data["reason"])
	}
	if got := event.Data["start_time"]; got != "15:00" {
		t.Fatalf("unexpected start_time metadata: %v", got)
	}
	if got := event.Data["start_time_text"]; got != "3 PM" {
		t.Fatalf("unexpected speech time: %v", got)
	}
	if !strings.Contains(event.Message, "was cancelled") {
		t.Fatalf("unexpected message: %s", event.Message)
	}
}
