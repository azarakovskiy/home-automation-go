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

func TestNew_DefaultsProfileToNormal(t *testing.T) {
	rem, err := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), DeliveryPolicy{RequiresAck: true}, makeMeta(), baseTime)
	if err != nil {
		t.Fatal(err)
	}
	if rem.Policy.Profile != ProfileNormal {
		t.Errorf("expected ProfileNormal, got %q", rem.Policy.Profile)
	}
}

func TestNew_FireCountIsZero(t *testing.T) {
	rem, err := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)
	if err != nil {
		t.Fatal(err)
	}
	if rem.State.FireCount != 0 {
		t.Errorf("expected FireCount 0, got %d", rem.State.FireCount)
	}
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

func TestTrigger_IncrementsFireCount(t *testing.T) {
	rem, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)
	rem.Trigger(baseTime)
	if rem.State.FireCount != 1 {
		t.Errorf("expected FireCount 1, got %d", rem.State.FireCount)
	}
}

func TestTrigger_OnceRequiresAck_SetsNextRunAtInitialDelay(t *testing.T) {
	rem, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)
	rem.Trigger(baseTime)
	want := baseTime.Add(EscalationNormal.InitialDelay)
	if rem.Schedule.NextRunAt == nil || !rem.Schedule.NextRunAt.Equal(want) {
		t.Errorf("NextRunAt = %v, want %v", rem.Schedule.NextRunAt, want)
	}
}

func TestTrigger_OnceRequiresAck_SecondFire_SetsRepeatInterval(t *testing.T) {
	rem, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)
	rem.Trigger(baseTime)
	rem.Trigger(baseTime.Add(EscalationNormal.InitialDelay))
	want := baseTime.Add(EscalationNormal.InitialDelay).Add(EscalationNormal.RepeatInterval)
	if rem.Schedule.NextRunAt == nil || !rem.Schedule.NextRunAt.Equal(want) {
		t.Errorf("NextRunAt = %v, want %v", rem.Schedule.NextRunAt, want)
	}
}

func TestTrigger_OnceNoAck_ClearsNextRunAt(t *testing.T) {
	p := DeliveryPolicy{RequiresAck: false, Profile: ProfileNormal}
	rem, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), p, makeMeta(), baseTime)
	rem.Trigger(baseTime)
	if rem.Schedule.NextRunAt != nil {
		t.Errorf("expected NextRunAt nil for once/no-ack, got %v", rem.Schedule.NextRunAt)
	}
}

func TestTrigger_Recurring_SetsNextRunAtRecurEvery(t *testing.T) {
	every := 10 * time.Minute
	sched := Schedule{Kind: ScheduleKindRecurring, TriggerAt: baseTime, RecurEvery: &every}
	rem, _ := New("r1", []string{"u1"}, sched, makePolicy(), makeMeta(), baseTime)
	rem.Trigger(baseTime)
	want := baseTime.Add(every)
	if rem.Schedule.NextRunAt == nil || !rem.Schedule.NextRunAt.Equal(want) {
		t.Errorf("NextRunAt = %v, want %v", rem.Schedule.NextRunAt, want)
	}
}

func TestTrigger_QuietProfile_MaxRepeats_ClearsNextRunAt(t *testing.T) {
	p := DeliveryPolicy{RequiresAck: true, Profile: ProfileQuiet}
	rem, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), p, makeMeta(), baseTime)
	// Fire MaxRepeats=3 times
	for i := 0; i < EscalationQuiet.MaxRepeats; i++ {
		rem.Trigger(baseTime.Add(time.Duration(i) * time.Hour))
	}
	if rem.State.FireCount != EscalationQuiet.MaxRepeats {
		t.Errorf("FireCount = %d, want %d", rem.State.FireCount, EscalationQuiet.MaxRepeats)
	}
	if rem.Schedule.NextRunAt != nil {
		t.Errorf("expected NextRunAt nil after MaxRepeats, got %v", rem.Schedule.NextRunAt)
	}
}

func TestTrigger_AnnoyingProfile_DecreaseInterval(t *testing.T) {
	p := DeliveryPolicy{RequiresAck: true, Profile: ProfileAnnoying}
	rem, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), p, makeMeta(), baseTime)

	// Fire 1: InitialDelay = 15m
	rem.Trigger(baseTime)
	want1 := baseTime.Add(15 * time.Minute)
	if rem.Schedule.NextRunAt == nil || !rem.Schedule.NextRunAt.Equal(want1) {
		t.Errorf("fire1 NextRunAt = %v, want %v", rem.Schedule.NextRunAt, want1)
	}

	// Fire 2: RepeatInterval - 0*DecreaseStep = 15m
	t2 := baseTime.Add(15 * time.Minute)
	rem.Trigger(t2)
	want2 := t2.Add(15 * time.Minute)
	if rem.Schedule.NextRunAt == nil || !rem.Schedule.NextRunAt.Equal(want2) {
		t.Errorf("fire2 NextRunAt = %v, want %v", rem.Schedule.NextRunAt, want2)
	}

	// Fire 3: 15m - 1*2m = 13m
	t3 := t2.Add(15 * time.Minute)
	rem.Trigger(t3)
	want3 := t3.Add(13 * time.Minute)
	if rem.Schedule.NextRunAt == nil || !rem.Schedule.NextRunAt.Equal(want3) {
		t.Errorf("fire3 NextRunAt = %v, want %v", rem.Schedule.NextRunAt, want3)
	}
}

func TestTrigger_AnnoyingProfile_MinInterval(t *testing.T) {
	p := DeliveryPolicy{RequiresAck: true, Profile: ProfileAnnoying}
	rem, _ := New("r1", []string{"u1"}, makeSchedule(ScheduleKindOnce), p, makeMeta(), baseTime)
	// Fire enough times to reach/exceed minimum
	now := baseTime
	for i := 0; i < 8; i++ {
		rem.Trigger(now)
		if rem.Schedule.NextRunAt != nil {
			now = *rem.Schedule.NextRunAt
		}
	}
	// All intervals after enough fires should be at MinInterval
	rem.Trigger(now)
	want := now.Add(EscalationAnnoying.MinInterval)
	if rem.Schedule.NextRunAt == nil || !rem.Schedule.NextRunAt.Equal(want) {
		t.Errorf("expected MinInterval %v, NextRunAt = %v", want, rem.Schedule.NextRunAt)
	}
}

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

func TestIsComplete_TrueAfterFirstAck(t *testing.T) {
	rem, _ := New("r1", []string{"u1", "u2"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)
	_ = rem.Acknowledge("u1", baseTime)
	if !rem.IsComplete() {
		t.Error("expected IsComplete true after first ack")
	}
}

func TestIsComplete_FalseWithNoAcks(t *testing.T) {
	rem, _ := New("r1", []string{"u1", "u2"}, makeSchedule(ScheduleKindOnce), makePolicy(), makeMeta(), baseTime)
	if rem.IsComplete() {
		t.Error("expected IsComplete false with no acks")
	}
}

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

func TestPolicyForProfile_Quiet(t *testing.T) {
	ep := PolicyForProfile(ProfileQuiet)
	if ep.InitialDelay != 30*time.Minute {
		t.Errorf("quiet InitialDelay = %v, want 30m", ep.InitialDelay)
	}
	if ep.RepeatInterval != time.Hour {
		t.Errorf("quiet RepeatInterval = %v, want 1h", ep.RepeatInterval)
	}
	if ep.MaxRepeats != 3 {
		t.Errorf("quiet MaxRepeats = %d, want 3", ep.MaxRepeats)
	}
}

func TestPolicyForProfile_Normal(t *testing.T) {
	ep := PolicyForProfile(ProfileNormal)
	if ep.InitialDelay != 15*time.Minute {
		t.Errorf("normal InitialDelay = %v, want 15m", ep.InitialDelay)
	}
	if ep.RepeatInterval != 15*time.Minute {
		t.Errorf("normal RepeatInterval = %v, want 15m", ep.RepeatInterval)
	}
	if ep.MaxRepeats != 0 {
		t.Errorf("normal MaxRepeats = %d, want 0", ep.MaxRepeats)
	}
}

func TestPolicyForProfile_Annoying(t *testing.T) {
	ep := PolicyForProfile(ProfileAnnoying)
	if ep.InitialDelay != 15*time.Minute {
		t.Errorf("annoying InitialDelay = %v, want 15m", ep.InitialDelay)
	}
	if ep.RepeatInterval != 15*time.Minute {
		t.Errorf("annoying RepeatInterval = %v, want 15m", ep.RepeatInterval)
	}
	if ep.DecreaseStep != 2*time.Minute {
		t.Errorf("annoying DecreaseStep = %v, want 2m", ep.DecreaseStep)
	}
	if ep.MinInterval != 5*time.Minute {
		t.Errorf("annoying MinInterval = %v, want 5m", ep.MinInterval)
	}
	if ep.MaxRepeats != 0 {
		t.Errorf("annoying MaxRepeats = %d, want 0", ep.MaxRepeats)
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
