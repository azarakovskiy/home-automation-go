package reminders_test

import (
	"context"
	"errors"
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

func newTestManager(t *testing.T) (*reminders.Manager, *mocks.MockRepository, *mocks.MockNotifier) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockRepository(ctrl)
	notifier := mocks.NewMockNotifier(ctrl)
	mgr := reminders.NewManager(repo, notifier, fixedNow(managerNow))
	return mgr, repo, notifier
}

func onceCmd(id string, at time.Time) reminders.CreateCommand {
	return reminders.CreateCommand{
		ID:      id,
		Targets: []string{"u1"},
		Schedule: reminders.Schedule{
			Kind:      reminders.ScheduleKindOnce,
			TriggerAt: at,
		},
		Policy: reminders.DeliveryPolicy{Profile: reminders.ProfileNormal},
		Meta:   reminders.Metadata{Message: "do it"},
	}
}

// --- Create ---

func TestManager_Create_SavesAndReturnsReminder(t *testing.T) {
	mgr, repo, _ := newTestManager(t)

	repo.EXPECT().
		Save(managerCtx, gomock.Any()).
		DoAndReturn(func(_ context.Context, r reminders.Reminder) error {
			if r.ID != "rem-1" {
				t.Errorf("expected ID rem-1, got %q", r.ID)
			}
			if r.State.FireCount != 0 {
				t.Errorf("expected FireCount 0 on create, got %d", r.State.FireCount)
			}
			return nil
		})

	rem, err := mgr.Create(managerCtx, onceCmd("rem-1", managerNow))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rem.ID != "rem-1" {
		t.Errorf("returned reminder ID = %q, want rem-1", rem.ID)
	}
}

func TestManager_Create_PropagatesRepoError(t *testing.T) {
	mgr, repo, _ := newTestManager(t)
	repo.EXPECT().Save(gomock.Any(), gomock.Any()).Return(errors.New("db down"))

	_, err := mgr.Create(managerCtx, onceCmd("r1", managerNow))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- Ack ---

func TestManager_Ack_Complete_Removes(t *testing.T) {
	mgr, repo, _ := newTestManager(t)

	stored := reminders.Reminder{
		ID:      "r1",
		Targets: []string{"u1"},
		Schedule: reminders.Schedule{Kind: reminders.ScheduleKindOnce, TriggerAt: managerNow},
		Policy:  reminders.DeliveryPolicy{RequiresAck: true, Profile: reminders.ProfileNormal},
		State:   reminders.State{CreatedAt: managerNow, UpdatedAt: managerNow},
		Meta:    reminders.Metadata{Message: "m"},
	}

	repo.EXPECT().GetByID(managerCtx, "r1").Return(stored, nil)
	repo.EXPECT().Remove(managerCtx, "r1").Return(nil)

	if err := mgr.Ack(managerCtx, "r1", "u1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestManager_Ack_MultiTarget_NotComplete_Saves(t *testing.T) {
	mgr, repo, _ := newTestManager(t)

	stored := reminders.Reminder{
		ID:      "r1",
		Targets: []string{"u1", "u2"},
		Schedule: reminders.Schedule{Kind: reminders.ScheduleKindOnce, TriggerAt: managerNow},
		Policy:  reminders.DeliveryPolicy{RequiresAck: true, Profile: reminders.ProfileNormal},
		State:   reminders.State{CreatedAt: managerNow, UpdatedAt: managerNow},
		Meta:    reminders.Metadata{Message: "m"},
	}

	// First ack from u1 still completes (any-ack policy)
	repo.EXPECT().GetByID(managerCtx, "r1").Return(stored, nil)
	repo.EXPECT().Remove(managerCtx, "r1").Return(nil)

	if err := mgr.Ack(managerCtx, "r1", "u1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestManager_Ack_NotTarget_Errors(t *testing.T) {
	mgr, repo, _ := newTestManager(t)

	stored := reminders.Reminder{
		ID:      "r1",
		Targets: []string{"u1"},
		Schedule: reminders.Schedule{Kind: reminders.ScheduleKindOnce, TriggerAt: managerNow},
		Policy:  reminders.DeliveryPolicy{RequiresAck: true, Profile: reminders.ProfileNormal},
		State:   reminders.State{CreatedAt: managerNow, UpdatedAt: managerNow},
		Meta:    reminders.Metadata{Message: "m"},
	}

	repo.EXPECT().GetByID(managerCtx, "r1").Return(stored, nil)

	err := mgr.Ack(managerCtx, "r1", "u-not-a-target")
	if !errors.Is(err, reminders.ErrNotTarget) {
		t.Errorf("expected ErrNotTarget, got %v", err)
	}
}

// --- Delete ---

func TestManager_Delete_Removes(t *testing.T) {
	mgr, repo, _ := newTestManager(t)
	repo.EXPECT().Remove(managerCtx, "r1").Return(nil)

	if err := mgr.Delete(managerCtx, "r1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Tick ---

func makeDueReminder(id string, requiresAck bool) reminders.Reminder {
	return reminders.Reminder{
		ID:      id,
		Targets: []string{"u1"},
		Schedule: reminders.Schedule{
			Kind:      reminders.ScheduleKindOnce,
			TriggerAt: managerNow.Add(-time.Minute),
		},
		Policy: reminders.DeliveryPolicy{RequiresAck: requiresAck, Profile: reminders.ProfileNormal},
		State:  reminders.State{CreatedAt: managerNow.Add(-time.Hour), UpdatedAt: managerNow.Add(-time.Hour)},
		Meta:   reminders.Metadata{Message: "do it"},
	}
}

func TestManager_Tick_Expired_Removes_NoNotify(t *testing.T) {
	mgr, repo, notifier := newTestManager(t)

	validUntil := managerNow.Add(-time.Second)
	rem := makeDueReminder("r1", false)
	rem.Schedule.ValidUntil = &validUntil

	repo.EXPECT().ListDueBefore(managerCtx, managerNow).Return([]reminders.Reminder{rem}, nil)
	repo.EXPECT().Remove(managerCtx, "r1").Return(nil)
	// notifier.Notify must NOT be called
	notifier.EXPECT().Notify(gomock.Any(), gomock.Any()).Times(0)

	if err := mgr.Tick(managerCtx, managerNow); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestManager_Tick_OnceNoAck_NotifiesAndRemoves(t *testing.T) {
	mgr, repo, notifier := newTestManager(t)

	rem := makeDueReminder("r1", false)
	repo.EXPECT().ListDueBefore(managerCtx, managerNow).Return([]reminders.Reminder{rem}, nil)
	notifier.EXPECT().Notify(managerCtx, gomock.Any()).Return(nil)
	repo.EXPECT().Remove(managerCtx, "r1").Return(nil)

	if err := mgr.Tick(managerCtx, managerNow); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestManager_Tick_OnceRequiresAck_NotifiesAndSaves(t *testing.T) {
	mgr, repo, notifier := newTestManager(t)

	rem := makeDueReminder("r1", true)
	repo.EXPECT().ListDueBefore(managerCtx, managerNow).Return([]reminders.Reminder{rem}, nil)
	notifier.EXPECT().Notify(managerCtx, gomock.Any()).Return(nil)
	repo.EXPECT().Save(managerCtx, gomock.Any()).Return(nil)

	if err := mgr.Tick(managerCtx, managerNow); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestManager_Tick_MaxRepeats_Removes_NoNotify(t *testing.T) {
	mgr, repo, notifier := newTestManager(t)

	// Quiet profile: MaxRepeats=3; pre-set FireCount to 2 so next Trigger hits limit
	rem := reminders.Reminder{
		ID:      "r1",
		Targets: []string{"u1"},
		Schedule: reminders.Schedule{
			Kind:      reminders.ScheduleKindOnce,
			TriggerAt: managerNow.Add(-time.Hour),
			NextRunAt: func() *time.Time { t := managerNow.Add(-time.Minute); return &t }(),
		},
		Policy: reminders.DeliveryPolicy{RequiresAck: true, Profile: reminders.ProfileQuiet},
		State: reminders.State{
			FireCount: 2, // next Trigger → FireCount=3 = MaxRepeats → silent remove
			CreatedAt: managerNow.Add(-3 * time.Hour),
			UpdatedAt: managerNow.Add(-3 * time.Hour),
		},
		Meta: reminders.Metadata{Message: "m"},
	}

	repo.EXPECT().ListDueBefore(managerCtx, managerNow).Return([]reminders.Reminder{rem}, nil)
	repo.EXPECT().Remove(managerCtx, "r1").Return(nil)
	notifier.EXPECT().Notify(gomock.Any(), gomock.Any()).Times(0)

	if err := mgr.Tick(managerCtx, managerNow); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestManager_Tick_Recurring_NotifiesAndSaves(t *testing.T) {
	mgr, repo, notifier := newTestManager(t)

	every := 10 * time.Minute
	rem := reminders.Reminder{
		ID:      "r1",
		Targets: []string{"u1"},
		Schedule: reminders.Schedule{
			Kind:       reminders.ScheduleKindRecurring,
			TriggerAt:  managerNow.Add(-time.Hour),
			RecurEvery: &every,
		},
		Policy: reminders.DeliveryPolicy{Profile: reminders.ProfileNormal},
		State:  reminders.State{CreatedAt: managerNow.Add(-time.Hour), UpdatedAt: managerNow.Add(-time.Hour)},
		Meta:   reminders.Metadata{Message: "recurring"},
	}

	repo.EXPECT().ListDueBefore(managerCtx, managerNow).Return([]reminders.Reminder{rem}, nil)
	notifier.EXPECT().Notify(managerCtx, gomock.Any()).Return(nil)
	repo.EXPECT().Save(managerCtx, gomock.Any()).Return(nil)

	if err := mgr.Tick(managerCtx, managerNow); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
