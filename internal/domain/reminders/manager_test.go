package reminders_test

import (
	"context"
	"testing"
	"time"

	"home-go/internal/domain/reminders"
	"home-go/internal/domain/reminders/mocks"

	"go.uber.org/mock/gomock"
)

var (
	managerNow = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	managerCtx = context.Background()
)

func fixedNow(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func newTestManager(t *testing.T) (*reminders.Manager, *mocks.MockRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockRepository(ctrl)
	mgr := reminders.NewManager(repo, fixedNow(managerNow))
	return mgr, repo
}

func onceSched(at time.Time) reminders.Schedule {
	return reminders.Schedule{
		Kind:      reminders.ScheduleKindOnce,
		TriggerAt: at,
	}
}

func recurSched(at time.Time, every time.Duration) reminders.Schedule {
	return reminders.Schedule{
		Kind:       reminders.ScheduleKindRecurring,
		TriggerAt:  at,
		RecurEvery: &every,
	}
}

func defaultPolicy() reminders.DeliveryPolicy {
	return reminders.DeliveryPolicy{
		RequiresAck:      false,
		CompletionPolicy: reminders.CompletionPolicyAllTargetsAck,
		Profile:          reminders.ProfileNormal,
	}
}

func ackPolicy() reminders.DeliveryPolicy {
	return reminders.DeliveryPolicy{
		RequiresAck:      true,
		CompletionPolicy: reminders.CompletionPolicyAllTargetsAck,
		Profile:          reminders.ProfileNormal,
	}
}

func anyAckPolicy() reminders.DeliveryPolicy {
	return reminders.DeliveryPolicy{
		RequiresAck:      true,
		CompletionPolicy: reminders.CompletionPolicyAnyTargetAck,
		Profile:          reminders.ProfileNormal,
	}
}

// --- Create ---

func TestManager_Create_SavesAndReturnsShowAction(t *testing.T) {
	mgr, repo := newTestManager(t)

	repo.EXPECT().
		Save(managerCtx, gomock.Any()).
		DoAndReturn(func(_ context.Context, r reminders.Reminder) error {
			if r.ID != "rem-1" {
				t.Errorf("expected ID rem-1, got %q", r.ID)
			}
			if r.State.Status != reminders.StatusActive {
				t.Errorf("expected active status, got %q", r.State.Status)
			}
			return nil
		})

	action, err := mgr.Create(managerCtx, "rem-1", []string{"alice"}, onceSched(managerNow), defaultPolicy(), reminders.Metadata{Message: "test"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if action.Kind != reminders.ActionShowProjection {
		t.Errorf("action kind: got %q, want %q", action.Kind, reminders.ActionShowProjection)
	}
	if action.Reminder.ID != "rem-1" {
		t.Errorf("action reminder ID: got %q, want rem-1", action.Reminder.ID)
	}
}

func TestManager_Create_FailsWithNoTargets(t *testing.T) {
	mgr, _ := newTestManager(t)

	_, err := mgr.Create(managerCtx, "rem-1", nil, onceSched(managerNow), defaultPolicy(), reminders.Metadata{})
	if err == nil {
		t.Fatal("expected error for no targets")
	}
}

// --- Delete ---

func TestManager_Delete_ReturnsRemoveAction(t *testing.T) {
	mgr, repo := newTestManager(t)

	existing := reminders.Reminder{
		ID:       "rem-2",
		Targets:  []string{"alice"},
		Acks:     []reminders.UserAck{},
		Schedule: onceSched(managerNow),
		Policy:   defaultPolicy(),
		State:    reminders.State{Status: reminders.StatusActive, CreatedAt: managerNow, UpdatedAt: managerNow},
	}

	repo.EXPECT().GetByID(managerCtx, "rem-2").Return(existing, nil)
	repo.EXPECT().Save(managerCtx, gomock.Any()).
		DoAndReturn(func(_ context.Context, r reminders.Reminder) error {
			if r.State.Status != reminders.StatusDeleted {
				t.Errorf("expected deleted status, got %q", r.State.Status)
			}
			return nil
		})

	action, err := mgr.Delete(managerCtx, "rem-2")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if action.Kind != reminders.ActionRemoveProjection {
		t.Errorf("action kind: got %q, want %q", action.Kind, reminders.ActionRemoveProjection)
	}
}

func TestManager_Delete_NotFound(t *testing.T) {
	mgr, repo := newTestManager(t)

	repo.EXPECT().GetByID(managerCtx, "missing").Return(reminders.Reminder{}, reminders.ErrNotFound)

	_, err := mgr.Delete(managerCtx, "missing")
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

// --- Ack ---

func TestManager_Ack_SingleUser_CompletesAndRemoves(t *testing.T) {
	mgr, repo := newTestManager(t)

	existing := reminders.Reminder{
		ID:       "rem-3",
		Targets:  []string{"alice"},
		Acks:     []reminders.UserAck{},
		Schedule: onceSched(managerNow.Add(-time.Hour)),
		Policy:   ackPolicy(),
		State:    reminders.State{Status: reminders.StatusActive, CreatedAt: managerNow, UpdatedAt: managerNow},
	}

	repo.EXPECT().GetByID(managerCtx, "rem-3").Return(existing, nil)
	repo.EXPECT().Save(managerCtx, gomock.Any()).
		DoAndReturn(func(_ context.Context, r reminders.Reminder) error {
			if r.State.Status != reminders.StatusCompleted {
				t.Errorf("expected completed, got %q", r.State.Status)
			}
			return nil
		})

	action, err := mgr.Ack(managerCtx, "rem-3", "alice")
	if err != nil {
		t.Fatalf("Ack: %v", err)
	}
	if action.Kind != reminders.ActionRemoveProjection {
		t.Errorf("action kind: got %q, want %q", action.Kind, reminders.ActionRemoveProjection)
	}
}

func TestManager_Ack_MultiUser_PartialAck_AllTargetsPolicy(t *testing.T) {
	mgr, repo := newTestManager(t)

	existing := reminders.Reminder{
		ID:       "rem-4",
		Targets:  []string{"alice", "bob"},
		Acks:     []reminders.UserAck{},
		Schedule: onceSched(managerNow.Add(-time.Hour)),
		Policy:   ackPolicy(), // all_targets_ack
		State:    reminders.State{Status: reminders.StatusActive, CreatedAt: managerNow, UpdatedAt: managerNow},
	}

	repo.EXPECT().GetByID(managerCtx, "rem-4").Return(existing, nil)
	repo.EXPECT().Save(managerCtx, gomock.Any()).
		DoAndReturn(func(_ context.Context, r reminders.Reminder) error {
			if r.State.Status != reminders.StatusActive {
				t.Errorf("expected active (partial ack), got %q", r.State.Status)
			}
			return nil
		})

	action, err := mgr.Ack(managerCtx, "rem-4", "alice")
	if err != nil {
		t.Fatalf("Ack: %v", err)
	}
	if action.Kind != reminders.ActionShowProjection {
		t.Errorf("action kind: got %q, want %q (partial ack should refresh)", action.Kind, reminders.ActionShowProjection)
	}
}

func TestManager_Ack_MultiUser_AllAck_AllTargetsPolicy(t *testing.T) {
	mgr, repo := newTestManager(t)

	aliceAckedAt := managerNow.Add(-5 * time.Minute)
	existing := reminders.Reminder{
		ID:       "rem-5",
		Targets:  []string{"alice", "bob"},
		Acks:     []reminders.UserAck{{UserID: "alice", AckedAt: aliceAckedAt}},
		Schedule: onceSched(managerNow.Add(-time.Hour)),
		Policy:   ackPolicy(), // all_targets_ack
		State:    reminders.State{Status: reminders.StatusActive, CreatedAt: managerNow, UpdatedAt: managerNow},
	}

	repo.EXPECT().GetByID(managerCtx, "rem-5").Return(existing, nil)
	repo.EXPECT().Save(managerCtx, gomock.Any()).
		DoAndReturn(func(_ context.Context, r reminders.Reminder) error {
			if r.State.Status != reminders.StatusCompleted {
				t.Errorf("expected completed (all acked), got %q", r.State.Status)
			}
			return nil
		})

	action, err := mgr.Ack(managerCtx, "rem-5", "bob")
	if err != nil {
		t.Fatalf("Ack: %v", err)
	}
	if action.Kind != reminders.ActionRemoveProjection {
		t.Errorf("action kind: got %q, want %q", action.Kind, reminders.ActionRemoveProjection)
	}
}

func TestManager_Ack_AnyTargetPolicy_CompletesOnFirstAck(t *testing.T) {
	mgr, repo := newTestManager(t)

	existing := reminders.Reminder{
		ID:       "rem-6",
		Targets:  []string{"alice", "bob"},
		Acks:     []reminders.UserAck{},
		Schedule: onceSched(managerNow.Add(-time.Hour)),
		Policy:   anyAckPolicy(), // any_target_ack
		State:    reminders.State{Status: reminders.StatusActive, CreatedAt: managerNow, UpdatedAt: managerNow},
	}

	repo.EXPECT().GetByID(managerCtx, "rem-6").Return(existing, nil)
	repo.EXPECT().Save(managerCtx, gomock.Any()).
		DoAndReturn(func(_ context.Context, r reminders.Reminder) error {
			if r.State.Status != reminders.StatusCompleted {
				t.Errorf("expected completed on first ack with any_target_ack, got %q", r.State.Status)
			}
			return nil
		})

	action, err := mgr.Ack(managerCtx, "rem-6", "alice")
	if err != nil {
		t.Fatalf("Ack: %v", err)
	}
	if action.Kind != reminders.ActionRemoveProjection {
		t.Errorf("action kind: got %q, want %q", action.Kind, reminders.ActionRemoveProjection)
	}
}

// --- Restore ---

func TestManager_Restore_ReturnsActiveReminders(t *testing.T) {
	mgr, repo := newTestManager(t)

	active := []reminders.Reminder{
		{ID: "rem-a", State: reminders.State{Status: reminders.StatusActive}},
		{ID: "rem-b", State: reminders.State{Status: reminders.StatusActive}},
	}

	repo.EXPECT().ListActive(managerCtx).Return(active, nil)

	got, err := mgr.Restore(managerCtx)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("Restore: got %d reminders, want 2", len(got))
	}
}

// --- Tick ---

func TestManager_Tick_OnceDueNoAck_CompletesAndRemoves(t *testing.T) {
	mgr, repo := newTestManager(t)

	due := reminders.Reminder{
		ID:       "rem-7",
		Targets:  []string{"alice"},
		Acks:     []reminders.UserAck{},
		Schedule: onceSched(managerNow.Add(-time.Minute)),
		Policy:   defaultPolicy(), // no ack required
		State:    reminders.State{Status: reminders.StatusActive, CreatedAt: managerNow, UpdatedAt: managerNow},
	}

	repo.EXPECT().ListDueBefore(managerCtx, managerNow).Return([]reminders.Reminder{due}, nil)
	repo.EXPECT().Save(managerCtx, gomock.Any()).
		DoAndReturn(func(_ context.Context, r reminders.Reminder) error {
			if r.State.Status != reminders.StatusCompleted {
				t.Errorf("expected completed, got %q", r.State.Status)
			}
			return nil
		})

	actions, err := mgr.Tick(managerCtx, managerNow)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(actions) != 1 || actions[0].Kind != reminders.ActionRemoveProjection {
		t.Errorf("expected 1 RemoveProjection action, got %v", actions)
	}
}

func TestManager_Tick_OnceWithAck_ShowsProjection(t *testing.T) {
	mgr, repo := newTestManager(t)

	due := reminders.Reminder{
		ID:       "rem-8",
		Targets:  []string{"alice"},
		Acks:     []reminders.UserAck{},
		Schedule: onceSched(managerNow.Add(-time.Minute)),
		Policy:   ackPolicy(), // requires ack
		State:    reminders.State{Status: reminders.StatusActive, CreatedAt: managerNow, UpdatedAt: managerNow},
	}

	repo.EXPECT().ListDueBefore(managerCtx, managerNow).Return([]reminders.Reminder{due}, nil)
	repo.EXPECT().Save(managerCtx, gomock.Any()).Return(nil)

	actions, err := mgr.Tick(managerCtx, managerNow)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(actions) != 1 || actions[0].Kind != reminders.ActionShowProjection {
		t.Errorf("expected 1 ShowProjection action, got %v", actions)
	}
}

func TestManager_Tick_Recurring_ShowsProjection(t *testing.T) {
	mgr, repo := newTestManager(t)

	every := time.Hour
	due := reminders.Reminder{
		ID:       "rem-9",
		Targets:  []string{"alice"},
		Acks:     []reminders.UserAck{},
		Schedule: recurSched(managerNow.Add(-time.Hour), every),
		Policy:   defaultPolicy(),
		State:    reminders.State{Status: reminders.StatusActive, CreatedAt: managerNow, UpdatedAt: managerNow},
	}

	repo.EXPECT().ListDueBefore(managerCtx, managerNow).Return([]reminders.Reminder{due}, nil)
	repo.EXPECT().Save(managerCtx, gomock.Any()).
		DoAndReturn(func(_ context.Context, r reminders.Reminder) error {
			if r.State.Status != reminders.StatusActive {
				t.Errorf("recurring should stay active, got %q", r.State.Status)
			}
			if r.Schedule.NextRunAt == nil {
				t.Error("NextRunAt should be set after trigger")
			}
			return nil
		})

	actions, err := mgr.Tick(managerCtx, managerNow)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(actions) != 1 || actions[0].Kind != reminders.ActionShowProjection {
		t.Errorf("expected 1 ShowProjection action, got %v", actions)
	}
}

func TestManager_Tick_ExpiredReminder_Expires(t *testing.T) {
	mgr, repo := newTestManager(t)

	past := managerNow.Add(-2 * time.Hour)
	validUntil := managerNow.Add(-time.Hour) // already expired
	every := time.Minute

	due := reminders.Reminder{
		ID:      "rem-exp",
		Targets: []string{"alice"},
		Acks:    []reminders.UserAck{},
		Schedule: reminders.Schedule{
			Kind:       reminders.ScheduleKindRecurring,
			TriggerAt:  past,
			NextRunAt:  &past,
			RecurEvery: &every,
			ValidUntil: &validUntil,
		},
		Policy: defaultPolicy(),
		State:  reminders.State{Status: reminders.StatusActive, CreatedAt: past, UpdatedAt: past},
	}

	repo.EXPECT().ListDueBefore(managerCtx, managerNow).Return([]reminders.Reminder{due}, nil)
	repo.EXPECT().Save(managerCtx, gomock.Any()).
		DoAndReturn(func(_ context.Context, r reminders.Reminder) error {
			if r.State.Status != reminders.StatusExpired {
				t.Errorf("expected expired, got %q", r.State.Status)
			}
			return nil
		})

	actions, err := mgr.Tick(managerCtx, managerNow)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(actions) != 1 || actions[0].Kind != reminders.ActionRemoveProjection {
		t.Errorf("expected 1 RemoveProjection for expired reminder, got %v", actions)
	}
}

func TestManager_Tick_NothingDue_ReturnsEmpty(t *testing.T) {
	mgr, repo := newTestManager(t)

	repo.EXPECT().ListDueBefore(managerCtx, managerNow).Return(nil, nil)

	actions, err := mgr.Tick(managerCtx, managerNow)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected no actions, got %d", len(actions))
	}
}
