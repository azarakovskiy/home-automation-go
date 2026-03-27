package dishwasher_test

import (
	"strings"
	"testing"
	"time"

	dishwasher "home-go/internal/domain/devices/dishwasher"
	"home-go/internal/domain/optimizer"
	"home-go/internal/domain/scheduler"
	dishwasher_mocks "home-go/internal/mocks/domain/devices/dishwasher"

	"go.uber.org/mock/gomock"
	ga "saml.dev/gome-assistant"
)

func TestHandleScheduleFlagChangeCancelsPendingSchedule(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm := dishwasher_mocks.NewMockScheduleStateStore(ctrl)
	d := dishwasher.NewTestDishwasher(sm)
	notifier := dishwasher.NewTestNotificationService()
	d.SetNotificationSenderForTest(notifier)
	startTime := time.Date(2025, time.October, 10, 21, 0, 0, 0, time.UTC)
	d.SetPendingScheduleForTest(&dishwasher.PendingSchedule{
		Mode:      dishwasher.ModeAuto,
		StartTime: startTime,
		Result:    &optimizer.OptimizationResult{},
	})

	sm.EXPECT().ClearSchedule().Return(nil)

	d.HandleScheduleFlagChangeForTest(ga.EntityData{FromState: "on", ToState: "off"})

	if d.PendingScheduleForTest() != nil {
		t.Fatal("expected pending schedule to be cleared")
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
	if got := event.Data["reason"]; got != "input_boolean turned off" {
		t.Fatalf("expected reason metadata, got %v", got)
	}
	if got := event.Data["start_time_text"]; got != "9 PM" {
		t.Fatalf("unexpected start time text: %v", got)
	}
}

func TestHandleScheduleRequestCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm := dishwasher_mocks.NewMockScheduleStateStore(ctrl)
	d := dishwasher.NewTestDishwasher(sm)
	notifier := dishwasher.NewTestNotificationService()
	d.SetNotificationSenderForTest(notifier)
	startTime := time.Date(2025, time.October, 11, 15, 0, 0, 0, time.UTC)
	d.SetPendingScheduleForTest(&dishwasher.PendingSchedule{
		Mode:      dishwasher.ModeAuto,
		StartTime: startTime,
		Result:    &optimizer.OptimizationResult{},
	})

	sm.EXPECT().ClearSchedule().Return(nil)

	d.HandleScheduleRequestForTest(scheduler.ScheduleRequest{
		Device: "dishwasher",
		Mode:   string(dishwasher.ModeCancel),
	})

	if d.PendingScheduleForTest() != nil {
		t.Fatal("expected pending schedule to be cleared by cancel request")
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
