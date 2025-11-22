package dishwasher

import (
	"strings"
	"testing"
	"time"
)

func TestAnnounceImmediateStart(t *testing.T) {
	notifier := NewTestNotificationService()
	d := NewTestDishwasher(nil)
	d.SetNotificationSenderForTest(notifier)

	startTime := time.Date(2025, time.October, 12, 21, 0, 0, 0, time.UTC)

	d.announceImmediateStart(startTime, 12)

	event, ok := notifier.LastEvent()
	if !ok {
		t.Fatal("expected immediate start announcement")
	}

	if event.Type != "started" {
		t.Fatalf("expected started notification, got %s", event.Type)
	}

	if got := event.Data["start_time"]; got != "21:00" {
		t.Fatalf("unexpected start time metadata: %v", got)
	}

	if got := event.Data["start_time_text"]; got != "9 PM" {
		t.Fatalf("unexpected speech time: %v", got)
	}

	if got := event.Data["savings_percent"]; got != float64(12) {
		t.Fatalf("unexpected savings metadata: %v", got)
	}

	if !strings.Contains(event.Message, "Dishwasher starts now") {
		t.Fatalf("unexpected message: %s", event.Message)
	}

	if !strings.Contains(event.Message, "saving 12 percent") {
		t.Fatalf("expected savings in message, got: %s", event.Message)
	}
}
