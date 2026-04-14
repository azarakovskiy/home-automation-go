package sqlite_test

import (
	"context"
	"testing"
	"time"

	"home-go/internal/config"
	"home-go/internal/domain/reminders"
	"home-go/internal/tech/sqlite"
)

// baseTime is a fixed reference time with second precision (no sub-seconds, since
// the DB stores Unix seconds and we compare with == after round-trip).
var baseTime = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

const testUserAlice = "alice"

func openDB(t *testing.T) *reminders.Repository {
	t.Helper()
	db, err := sqlite.Open(config.DatabaseConfig{Path: ":memory:"})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	repo := sqlite.NewRemindersRepo(db)
	var r reminders.Repository = repo
	return &r
}

func newOnceReminder(id string, triggerAt time.Time) reminders.Reminder {
	return reminders.Reminder{
		ID:      id,
		Targets: []string{"user1"},
		Acks:    []reminders.UserAck{},
		Schedule: reminders.Schedule{
			Kind:      reminders.ScheduleKindOnce,
			TriggerAt: triggerAt,
		},
		Policy: reminders.DeliveryPolicy{
			RequiresAck:      false,
			CompletionPolicy: reminders.CompletionPolicyAllTargetsAck,
			Profile:          reminders.ProfileNormal,
		},
		State: reminders.State{
			Status:    reminders.StatusActive,
			CreatedAt: baseTime,
			UpdatedAt: baseTime,
		},
		Meta: reminders.Metadata{
			Source:  "test",
			Owner:   "owner1",
			Message: "hello",
		},
	}
}

func assertReminderSchedule(t *testing.T, got, want reminders.Schedule) {
	t.Helper()
	if got.Kind != want.Kind {
		t.Errorf("Kind: got %q, want %q", got.Kind, want.Kind)
	}
	if !got.TriggerAt.Equal(want.TriggerAt) {
		t.Errorf("TriggerAt: got %v, want %v", got.TriggerAt, want.TriggerAt)
	}
	if got.NextRunAt == nil || !got.NextRunAt.Equal(*want.NextRunAt) {
		t.Errorf("NextRunAt: got %v, want %v", got.NextRunAt, want.NextRunAt)
	}
	if got.RecurEvery == nil || *got.RecurEvery != *want.RecurEvery {
		t.Errorf("RecurEvery: got %v, want %v", got.RecurEvery, want.RecurEvery)
	}
	if got.ValidUntil == nil || !got.ValidUntil.Equal(*want.ValidUntil) {
		t.Errorf("ValidUntil: got %v, want %v", got.ValidUntil, want.ValidUntil)
	}
}

func assertReminderPolicy(t *testing.T, got, want reminders.DeliveryPolicy) {
	t.Helper()
	if got.RequiresAck != want.RequiresAck {
		t.Errorf("RequiresAck: got %v, want %v", got.RequiresAck, want.RequiresAck)
	}
	if got.CompletionPolicy != want.CompletionPolicy {
		t.Errorf("CompletionPolicy: got %q, want %q", got.CompletionPolicy, want.CompletionPolicy)
	}
	if got.Profile != want.Profile {
		t.Errorf("Profile: got %q, want %q", got.Profile, want.Profile)
	}
}

func assertReminderState(t *testing.T, got, want reminders.State) {
	t.Helper()
	if got.Status != want.Status {
		t.Errorf("Status: got %q, want %q", got.Status, want.Status)
	}
	if got.LastFiredAt == nil || !got.LastFiredAt.Equal(*want.LastFiredAt) {
		t.Errorf("LastFiredAt: got %v, want %v", got.LastFiredAt, want.LastFiredAt)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, want.CreatedAt)
	}
}

func assertReminderMeta(t *testing.T, got, want reminders.Metadata) {
	t.Helper()
	if got.Source != want.Source {
		t.Errorf("Source: got %q, want %q", got.Source, want.Source)
	}
	if got.Owner != want.Owner {
		t.Errorf("Owner: got %q, want %q", got.Owner, want.Owner)
	}
	if got.Message != want.Message {
		t.Errorf("Message: got %q, want %q", got.Message, want.Message)
	}
}

func TestSaveAndGetByID_RoundTrip(t *testing.T) {
	repoPtr := openDB(t)
	repo := *repoPtr
	ctx := context.Background()

	nextRun := baseTime.Add(time.Hour)
	recurEvery := 30 * time.Minute
	validUntil := baseTime.Add(24 * time.Hour)
	lastFired := baseTime.Add(-time.Hour)

	rem := reminders.Reminder{
		ID:      "rem-1",
		Targets: []string{testUserAlice, "bob"},
		Acks: []reminders.UserAck{
			{UserID: testUserAlice, AckedAt: baseTime},
		},
		Schedule: reminders.Schedule{
			Kind:       reminders.ScheduleKindRecurring,
			TriggerAt:  baseTime,
			NextRunAt:  &nextRun,
			RecurEvery: &recurEvery,
			ValidUntil: &validUntil,
		},
		Policy: reminders.DeliveryPolicy{
			RequiresAck:      true,
			CompletionPolicy: reminders.CompletionPolicyAnyTargetAck,
			Profile:          reminders.ProfileAnnoying,
		},
		State: reminders.State{
			Status:      reminders.StatusActive,
			LastFiredAt: &lastFired,
			CreatedAt:   baseTime,
			UpdatedAt:   baseTime,
		},
		Meta: reminders.Metadata{
			Source:  "scheduler",
			Owner:   "owner1",
			Message: "take your meds",
		},
	}

	if err := repo.Save(ctx, rem); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.GetByID(ctx, "rem-1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if got.ID != rem.ID {
		t.Errorf("ID: got %q, want %q", got.ID, rem.ID)
	}
	assertReminderSchedule(t, got.Schedule, rem.Schedule)
	assertReminderPolicy(t, got.Policy, rem.Policy)
	assertReminderState(t, got.State, rem.State)
	assertReminderMeta(t, got.Meta, rem.Meta)

	// Targets (sorted)
	if len(got.Targets) != 2 || got.Targets[0] != testUserAlice || got.Targets[1] != "bob" {
		t.Errorf("Targets: got %v, want [alice bob]", got.Targets)
	}

	// Acks
	if len(got.Acks) != 1 || got.Acks[0].UserID != testUserAlice || !got.Acks[0].AckedAt.Equal(baseTime) {
		t.Errorf("Acks: got %v", got.Acks)
	}
}

func TestListActive_FiltersStatus(t *testing.T) {
	repoPtr := openDB(t)
	repo := *repoPtr
	ctx := context.Background()

	active := newOnceReminder("active-1", baseTime)
	active.State.Status = reminders.StatusActive

	completed := newOnceReminder("completed-1", baseTime)
	completed.State.Status = reminders.StatusCompleted

	deleted := newOnceReminder("deleted-1", baseTime)
	deleted.State.Status = reminders.StatusDeleted

	expired := newOnceReminder("expired-1", baseTime)
	expired.State.Status = reminders.StatusExpired

	for _, r := range []reminders.Reminder{active, completed, deleted, expired} {
		if err := repo.Save(ctx, r); err != nil {
			t.Fatalf("Save %s: %v", r.ID, err)
		}
	}

	list, err := repo.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	if len(list) != 1 || list[0].ID != "active-1" {
		t.Errorf("ListActive: got %v IDs, want [active-1]", idsOf(list))
	}
}

func TestListDueBefore_TriggerAt(t *testing.T) {
	repoPtr := openDB(t)
	repo := *repoPtr
	ctx := context.Background()

	early := newOnceReminder("early", baseTime.Add(-time.Hour))
	late := newOnceReminder("late", baseTime.Add(time.Hour))

	for _, r := range []reminders.Reminder{early, late} {
		if err := repo.Save(ctx, r); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	due, err := repo.ListDueBefore(ctx, baseTime)
	if err != nil {
		t.Fatalf("ListDueBefore: %v", err)
	}
	if len(due) != 1 || due[0].ID != "early" {
		t.Errorf("ListDueBefore: got %v, want [early]", idsOf(due))
	}
}

func TestListDueBefore_NextRunAt(t *testing.T) {
	repoPtr := openDB(t)
	repo := *repoPtr
	ctx := context.Background()

	// Recurring reminder with next_run_at set
	nextEarly := baseTime.Add(-30 * time.Minute)
	nextLate := baseTime.Add(30 * time.Minute)
	recurEvery := time.Hour

	earlyRecur := newOnceReminder("recur-early", baseTime)
	earlyRecur.Schedule.Kind = reminders.ScheduleKindRecurring
	earlyRecur.Schedule.NextRunAt = &nextEarly
	earlyRecur.Schedule.RecurEvery = &recurEvery

	lateRecur := newOnceReminder("recur-late", baseTime)
	lateRecur.Schedule.Kind = reminders.ScheduleKindRecurring
	lateRecur.Schedule.NextRunAt = &nextLate
	lateRecur.Schedule.RecurEvery = &recurEvery

	for _, r := range []reminders.Reminder{earlyRecur, lateRecur} {
		if err := repo.Save(ctx, r); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}

	due, err := repo.ListDueBefore(ctx, baseTime)
	if err != nil {
		t.Fatalf("ListDueBefore: %v", err)
	}
	if len(due) != 1 || due[0].ID != "recur-early" {
		t.Errorf("ListDueBefore: got %v, want [recur-early]", idsOf(due))
	}
}

func TestMultiTarget_PreservedAcrossRoundTrip(t *testing.T) {
	repoPtr := openDB(t)
	repo := *repoPtr
	ctx := context.Background()

	rem := newOnceReminder("multi", baseTime)
	rem.Targets = []string{"alice", "bob", "charlie"}

	if err := repo.Save(ctx, rem); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.GetByID(ctx, "multi")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if len(got.Targets) != 3 {
		t.Fatalf("Targets: got %d, want 3", len(got.Targets))
	}
	// Targets are sorted
	want := []string{"alice", "bob", "charlie"}
	for i, u := range want {
		if got.Targets[i] != u {
			t.Errorf("Targets[%d]: got %q, want %q", i, got.Targets[i], u)
		}
	}
}

func TestAcks_SaveLoadUpdateIdempotent(t *testing.T) {
	repoPtr := openDB(t)
	repo := *repoPtr
	ctx := context.Background()

	rem := newOnceReminder("ack-test", baseTime)
	rem.Targets = []string{testUserAlice, "bob"}
	rem.Policy.RequiresAck = true

	// Save with no acks
	if err := repo.Save(ctx, rem); err != nil {
		t.Fatalf("Save (no acks): %v", err)
	}

	got, err := repo.GetByID(ctx, "ack-test")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(got.Acks) != 0 {
		t.Errorf("expected 0 acks, got %d", len(got.Acks))
	}

	// Save with one ack
	ackTime := baseTime.Add(5 * time.Minute)
	rem.Acks = []reminders.UserAck{{UserID: testUserAlice, AckedAt: ackTime}}
	if err := repo.Save(ctx, rem); err != nil {
		t.Fatalf("Save (one ack): %v", err)
	}

	got, err = repo.GetByID(ctx, "ack-test")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(got.Acks) != 1 || got.Acks[0].UserID != testUserAlice || !got.Acks[0].AckedAt.Equal(ackTime) {
		t.Errorf("Acks after first save: %v", got.Acks)
	}

	// Idempotent upsert: resave same ack with updated time
	updatedAckTime := ackTime.Add(time.Minute)
	rem.Acks[0].AckedAt = updatedAckTime
	if err := repo.Save(ctx, rem); err != nil {
		t.Fatalf("Save (updated ack): %v", err)
	}

	got, err = repo.GetByID(ctx, "ack-test")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(got.Acks) != 1 || !got.Acks[0].AckedAt.Equal(updatedAckTime) {
		t.Errorf("Acks after update: %v", got.Acks)
	}
}

func TestDelete_RemovesFromListActive(t *testing.T) {
	repoPtr := openDB(t)
	repo := *repoPtr
	ctx := context.Background()

	rem := newOnceReminder("del-1", baseTime)
	if err := repo.Save(ctx, rem); err != nil {
		t.Fatalf("Save: %v", err)
	}

	list, err := repo.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive before delete: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 active reminder before delete, got %d", len(list))
	}

	if err := repo.Delete(ctx, "del-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	list, err = repo.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive after delete: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 active reminders after delete, got %d", len(list))
	}

	// GetByID still works, status is deleted
	got, err := repo.GetByID(ctx, "del-1")
	if err != nil {
		t.Fatalf("GetByID after delete: %v", err)
	}
	if got.State.Status != reminders.StatusDeleted {
		t.Errorf("Status after delete: got %q, want %q", got.State.Status, reminders.StatusDeleted)
	}
}

func TestExpired_RemovesFromListActive(t *testing.T) {
	repoPtr := openDB(t)
	repo := *repoPtr
	ctx := context.Background()

	rem := newOnceReminder("exp-1", baseTime)
	rem.State.Status = reminders.StatusExpired
	if err := repo.Save(ctx, rem); err != nil {
		t.Fatalf("Save: %v", err)
	}

	list, err := repo.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 active reminders for expired, got %d", len(list))
	}
}

func TestGetByID_NotFound(t *testing.T) {
	repoPtr := openDB(t)
	repo := *repoPtr
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != reminders.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func idsOf(rs []reminders.Reminder) []string {
	ids := make([]string, len(rs))
	for i, r := range rs {
		ids[i] = r.ID
	}
	return ids
}
