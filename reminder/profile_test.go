package reminder

import (
	"testing"
	"time"
)

func TestNextIntervalDurationAnnoying(t *testing.T) {
	cfg := ReminderDefinition{
		InitialRepeatMin: DefaultInitialRepeatMinutes,
		MinRepeatMin:     DefaultMinRepeatMinutes,
		MaxRepeatMin:     DefaultMaxRepeatMinutes,
	}

	interval, cancel := nextIntervalDuration(ProfileAnnoying, cfg, cfg.InitialRepeatMin)
	if cancel {
		t.Fatalf("annoying profile should not cancel")
	}
	if interval != 10*time.Minute {
		t.Fatalf("expected 10m, got %s", interval)
	}

	interval, cancel = nextIntervalDuration(ProfileAnnoying, cfg, 10)
	if cancel {
		t.Fatalf("annoying profile should not cancel")
	}
	if interval != 5*time.Minute {
		t.Fatalf("expected 5m, got %s", interval)
	}

	interval, cancel = nextIntervalDuration(ProfileAnnoying, cfg, 5)
	if cancel {
		t.Fatalf("annoying profile should not cancel")
	}
	if interval != 3*time.Minute {
		t.Fatalf("expected 3m, got %s", interval)
	}

	interval, cancel = nextIntervalDuration(ProfileAnnoying, cfg, 3)
	if cancel {
		t.Fatalf("annoying profile should not cancel")
	}
	if interval != 2*time.Minute {
		t.Fatalf("expected clamp to 2m, got %s", interval)
	}
}

func TestNextIntervalDurationQuiet(t *testing.T) {
	cfg := ReminderDefinition{
		InitialRepeatMin: DefaultInitialRepeatMinutes,
		MinRepeatMin:     DefaultMinRepeatMinutes,
		MaxRepeatMin:     DefaultMaxRepeatMinutes,
	}

	prev := cfg.InitialRepeatMin
	expected := []time.Duration{
		30 * time.Minute,
		45 * time.Minute,
		68 * time.Minute,
	}

	for _, want := range expected {
		interval, cancel := nextIntervalDuration(ProfileQuiet, cfg, prev)
		if cancel {
			t.Fatalf("quiet should not cancel yet")
		}
		if interval != want {
			t.Fatalf("expected %s, got %s", want, interval)
		}
		prev = int(interval.Minutes())
	}

	// Next interval should trigger cancellation once exceeding max (180m)
	prev = 153
	interval, cancel := nextIntervalDuration(ProfileQuiet, cfg, prev)
	if !cancel {
		t.Fatalf("expected cancellation after exceeding 3h cap")
	}
	if interval != 0 {
		t.Fatalf("cancelled reminders should not have interval, got %s", interval)
	}
}

func TestNextIntervalDurationNormal(t *testing.T) {
	cfg := ReminderDefinition{InitialRepeatMin: 25}
	interval, cancel := nextIntervalDuration(ProfileNormal, cfg, 0)
	if cancel {
		t.Fatalf("normal profile should not cancel")
	}
	if interval != 25*time.Minute {
		t.Fatalf("expected 25m, got %s", interval)
	}
}
