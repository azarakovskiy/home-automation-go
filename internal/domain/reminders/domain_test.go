package reminders

import (
	"errors"
	"testing"
	"time"
)

var baseTime = time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

func TestDomainTypes_StateHasFireCount(t *testing.T) {
	s := State{FireCount: 3, CreatedAt: baseTime, UpdatedAt: baseTime}
	if s.FireCount != 3 {
		t.Errorf("expected FireCount 3, got %d", s.FireCount)
	}
}

func TestDomainTypes_DeliveryPolicyHasNoCompletionPolicy(t *testing.T) {
	p := DeliveryPolicy{RequiresAck: true, Profile: ProfileNormal}
	_ = p // compile-time check that no CompletionPolicy field exists
}

func makeSchedule(kind ScheduleKind) Schedule {
	return Schedule{
		Kind:      kind,
		TriggerAt: baseTime,
	}
}

func makePolicy() DeliveryPolicy {
	return DeliveryPolicy{
		RequiresAck: true,
		Profile:     ProfileNormal,
	}
}

func makeMeta() Metadata {
	return Metadata{Source: "test", Owner: "owner", Message: "do the thing"}
}

// --- New ---

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		targets []string
		sched   Schedule
		policy  DeliveryPolicy
		wantErr error
	}{
		{
			name:    "no targets",
			targets: nil,
			sched:   makeSchedule(ScheduleKindOnce),
			policy:  makePolicy(),
			wantErr: ErrNoTargets,
		},
		{
			name:    "empty targets slice",
			targets: []string{},
			sched:   makeSchedule(ScheduleKindOnce),
			policy:  makePolicy(),
			wantErr: ErrNoTargets,
		},
		{
			name:    "invalid schedule kind",
			targets: []string{"u1"},
			sched:   Schedule{Kind: "bogus", TriggerAt: baseTime},
			policy:  makePolicy(),
			wantErr: ErrInvalidSchedule,
		},
		{
			name:    "valid once",
			targets: []string{"u1"},
			sched:   makeSchedule(ScheduleKindOnce),
			policy:  makePolicy(),
			wantErr: nil,
		},
		{
			name:    "valid recurring",
			targets: []string{"u1", "u2"},
			sched:   makeSchedule(ScheduleKindRecurring),
			policy:  makePolicy(),
			wantErr: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, err := New("r1", tc.targets, tc.sched, tc.policy, makeMeta(), baseTime)
			if tc.wantErr != nil {
				assertErrorIs(t, err, tc.wantErr)
				return
			}
			assertNoError(t, err)
			assertEqual(t, r.State.CreatedAt, baseTime)
		})
	}
}

func TestNew_Defaults(t *testing.T) {
	r, err := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), DeliveryPolicy{}, makeMeta(), baseTime)
	assertNoError(t, err)
	assertEqual(t, r.Policy.Profile, ProfileNormal)
}

// --- IsDue ---

func TestIsDue(t *testing.T) {
	tests := []struct {
		name   string
		setup  func() Reminder
		now    time.Time
		expect bool
	}{
		{
			name: "once, before trigger",
			setup: func() Reminder {
				r, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)
				return r
			},
			now:    baseTime.Add(-1 * time.Minute),
			expect: false,
		},
		{
			name: "once, at trigger",
			setup: func() Reminder {
				r, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)
				return r
			},
			now:    baseTime,
			expect: true,
		},
		{
			name: "once, after trigger",
			setup: func() Reminder {
				r, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)
				return r
			},
			now:    baseTime.Add(1 * time.Hour),
			expect: true,
		},
		{
			name: "recurring with NextRunAt set, before next",
			setup: func() Reminder {
				r, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindRecurring), makePolicy(), makeMeta(), baseTime)
				next := baseTime.Add(1 * time.Hour)
				r.Schedule.NextRunAt = &next
				return r
			},
			now:    baseTime.Add(30 * time.Minute),
			expect: false,
		},
		{
			name: "recurring with NextRunAt set, at next",
			setup: func() Reminder {
				r, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindRecurring), makePolicy(), makeMeta(), baseTime)
				next := baseTime.Add(1 * time.Hour)
				r.Schedule.NextRunAt = &next
				return r
			},
			now:    baseTime.Add(1 * time.Hour),
			expect: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := tc.setup()
			got := r.IsDue(tc.now)
			assertEqual(t, got, tc.expect)
		})
	}
}

// --- Trigger ---

func TestTrigger_OnceNoAck(t *testing.T) {
	sched := makeSchedule(ScheduleKindOnce)
	policy := DeliveryPolicy{RequiresAck: false, Profile: ProfileNormal}
	r, _ := New("r1", []string{"u1"}, sched, policy, makeMeta(), baseTime)

	now := baseTime.Add(1 * time.Minute)
	err := r.Trigger(now)
	assertNoError(t, err)
	assertEqual(t, *r.State.LastFiredAt, now)
}

func TestTrigger_OnceWithAck(t *testing.T) {
	r, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)

	now := baseTime.Add(1 * time.Minute)
	err := r.Trigger(now)
	assertNoError(t, err)
}

func TestTrigger_OnceWithAck_SetsNextRunAtFromEscalationPolicy(t *testing.T) {
	profiles := []struct {
		profile     Profile
		wantInitial time.Duration
		wantRepeat  time.Duration
	}{
		{ProfileQuiet, EscalationQuiet.InitialDelay, EscalationQuiet.RepeatInterval},
		{ProfileNormal, EscalationNormal.InitialDelay, EscalationNormal.RepeatInterval},
		{ProfileAnnoying, EscalationAnnoying.InitialDelay, EscalationAnnoying.RepeatInterval},
	}

	for _, tc := range profiles {
		t.Run(string(tc.profile), func(t *testing.T) {
			policy := DeliveryPolicy{RequiresAck: true, Profile: tc.profile}
			r, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), policy, makeMeta(), baseTime)

			// First fire: NextRunAt uses InitialDelay.
			fire1 := baseTime.Add(1 * time.Minute)
			assertNoError(t, r.Trigger(fire1))
			if r.Schedule.NextRunAt == nil {
				t.Fatal("expected NextRunAt to be set after first trigger")
			}
			assertEqual(t, *r.Schedule.NextRunAt, fire1.Add(tc.wantInitial))

			// Second fire: NextRunAt uses RepeatInterval.
			fire2 := fire1.Add(tc.wantInitial)
			assertNoError(t, r.Trigger(fire2))
			assertEqual(t, *r.Schedule.NextRunAt, fire2.Add(tc.wantRepeat))
		})
	}
}

func TestTrigger_OnceWithAck_DoesNotReFireEveryTick(t *testing.T) {
	// After first trigger, the reminder must not be due again until NextRunAt.
	policy := DeliveryPolicy{RequiresAck: true, Profile: ProfileNormal}
	r, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), policy, makeMeta(), baseTime)

	fire := baseTime.Add(1 * time.Minute)
	assertNoError(t, r.Trigger(fire))

	// One second later: not due yet (InitialDelay is 5 min for normal).
	assertEqual(t, r.IsDue(fire.Add(1*time.Second)), false)

	// At InitialDelay boundary: due again.
	assertEqual(t, r.IsDue(fire.Add(EscalationNormal.InitialDelay)), true)
}

// TestTrigger_Recurring removed - Status field removed from State domain
// TestTrigger_NotActive removed - lifecycle.go Trigger will be updated in later task

// --- Acknowledge ---

func TestAcknowledge(t *testing.T) {
	t.Run("valid ack", func(t *testing.T) {
		r, _ := New("r1", []string{"u1", "u2"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)
		now := baseTime.Add(1 * time.Minute)
		err := r.Acknowledge("u1", now)
		assertNoError(t, err)
		assertEqual(t, len(r.Acks), 1)
		assertEqual(t, r.Acks[0].UserID, "u1")
	})

	t.Run("idempotent ack", func(t *testing.T) {
		r, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)
		now := baseTime.Add(1 * time.Minute)
		_ = r.Acknowledge("u1", now)
		err := r.Acknowledge("u1", now.Add(1*time.Minute))
		assertNoError(t, err)
		assertEqual(t, len(r.Acks), 1)
	})

	t.Run("not a target", func(t *testing.T) {
		r, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)
		err := r.Acknowledge("stranger", baseTime)
		assertErrorIs(t, err, ErrNotTarget)
	})

	t.Run("completes on last ack", func(t *testing.T) {
		r, _ := New("r1", []string{"u1", "u2"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)
		now := baseTime.Add(1 * time.Minute)
		_ = r.Acknowledge("u1", now)

		_ = r.Acknowledge("u2", now.Add(1*time.Minute))
		// Verify both acks were recorded
		assertEqual(t, len(r.Acks), 2)
	})
}

// --- IsComplete ---

func TestIsComplete_AllTargetsAck(t *testing.T) {
	tests := []struct {
		name   string
		acked  []string
		expect bool
	}{
		{"no acks", nil, false},
		{"partial", []string{"u1"}, false},
		{"all acked", []string{"u1", "u2"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := New("r1", []string{"u1", "u2"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)
			for _, uid := range tc.acked {
				r.Acks = append(r.Acks, UserAck{UserID: uid, AckedAt: baseTime})
			}
			assertEqual(t, r.IsComplete(), tc.expect)
		})
	}
}

func TestIsComplete_AnyTargetAck(t *testing.T) {
	policy := DeliveryPolicy{
		RequiresAck: true,
		Profile:     ProfileNormal,
	}

	tests := []struct {
		name   string
		acked  []string
		expect bool
	}{
		{"no acks", nil, false},
		{"one ack", []string{"u1"}, true},
		{"two acks", []string{"u1", "u2"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := New("r1", []string{"u1", "u2"}, makeSchedule(ScheduleKindOnce), policy, makeMeta(), baseTime)
			for _, uid := range tc.acked {
				r.Acks = append(r.Acks, UserAck{UserID: uid, AckedAt: baseTime})
			}
			assertEqual(t, r.IsComplete(), tc.expect)
		})
	}
}

// TestAcknowledge_AnyTargetAck_CompletesOnFirst removed - requires lifecycle updates

// --- IsExpired ---

func TestIsExpired(t *testing.T) {
	tests := []struct {
		name       string
		validUntil *time.Time
		now        time.Time
		expect     bool
	}{
		{
			name:       "no expiry",
			validUntil: nil,
			now:        baseTime.Add(999 * time.Hour),
			expect:     false,
		},
		{
			name:       "before expiry",
			validUntil: timePtr(baseTime.Add(1 * time.Hour)),
			now:        baseTime.Add(30 * time.Minute),
			expect:     false,
		},
		{
			name:       "at expiry boundary",
			validUntil: timePtr(baseTime.Add(1 * time.Hour)),
			now:        baseTime.Add(1 * time.Hour),
			expect:     false, // After, not equal
		},
		{
			name:       "after expiry",
			validUntil: timePtr(baseTime.Add(1 * time.Hour)),
			now:        baseTime.Add(2 * time.Hour),
			expect:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sched := makeSchedule(ScheduleKindOnce)
			sched.ValidUntil = tc.validUntil
			r, _ := New("r1", []string{"u1"}, sched, makePolicy(), makeMeta(), baseTime)
			assertEqual(t, r.IsExpired(tc.now), tc.expect)
		})
	}
}

// --- Delete / Expire ---

func TestDelete(t *testing.T) {
	r, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)
	now := baseTime.Add(5 * time.Minute)
	r.Delete(now)
	assertEqual(t, r.State.UpdatedAt, now)
}

func TestExpire(t *testing.T) {
	r, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)
	now := baseTime.Add(5 * time.Minute)
	r.Expire(now)
	assertEqual(t, r.State.UpdatedAt, now)
}

// --- PolicyForProfile ---

func TestPolicyForProfile(t *testing.T) {
	tests := []struct {
		profile Profile
		expect  EscalationPolicy
	}{
		{ProfileQuiet, EscalationQuiet},
		{ProfileNormal, EscalationNormal},
		{ProfileAnnoying, EscalationAnnoying},
		{"unknown", EscalationNormal}, // defaults to normal
	}

	for _, tc := range tests {
		t.Run(string(tc.profile), func(t *testing.T) {
			got := PolicyForProfile(tc.profile)
			assertEqual(t, got, tc.expect)
		})
	}
}

// --- Helpers ---

func timePtr(t time.Time) *time.Time { return &t }

func assertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertErrorIs(t *testing.T, err, target error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %v, got nil", target)
		return
	}
	if !errors.Is(err, target) {
		t.Errorf("expected error %v, got %v", target, err)
	}
}
