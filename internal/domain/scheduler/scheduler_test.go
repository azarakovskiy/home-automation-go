package scheduler

import (
	"errors"
	"testing"
	"time"
)

func TestSchedulerScheduleAndRestore(t *testing.T) {
	store := &fakeStore{}
	s := New(store, &fakeRunner{})
	start := time.Date(2026, time.March, 29, 21, 0, 0, 0, time.UTC)

	if err := s.Schedule(Plan{StartTime: start}); err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	if !s.HasPending() {
		t.Fatal("expected pending schedule after Schedule")
	}
	if store.saved == nil || !store.saved.StartTime.Equal(start) {
		t.Fatalf("saved plan = %+v, want start time %s", store.saved, start)
	}

	restored := New(&fakeStore{restored: &Plan{StartTime: start}}, &fakeRunner{})
	if err := restored.Restore(start.Add(-time.Minute)); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if !restored.HasPending() {
		t.Fatal("expected pending schedule after Restore")
	}
}

func TestSchedulerTickStartsAndClears(t *testing.T) {
	store := &fakeStore{}
	runner := &fakeRunner{}
	s := New(store, runner)
	start := time.Date(2026, time.March, 29, 21, 0, 0, 0, time.UTC)

	if err := s.Schedule(Plan{StartTime: start}); err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	if err := s.Tick(start); err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if runner.startCalls != 1 {
		t.Fatalf("startCalls = %d, want 1", runner.startCalls)
	}
	if store.clearCalls != 1 {
		t.Fatalf("clearCalls = %d, want 1", store.clearCalls)
	}
	if s.HasPending() {
		t.Fatal("expected no pending schedule after Tick")
	}
}

func TestSchedulerRestoreHandlesExpiredSchedule(t *testing.T) {
	store := &fakeStore{
		restored: &Plan{StartTime: time.Date(2026, time.March, 29, 21, 0, 0, 0, time.UTC)},
	}
	runner := &fakeRunner{}
	s := New(store, runner)

	if err := s.Restore(time.Date(2026, time.March, 29, 21, 1, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if runner.expiredCalls != 1 {
		t.Fatalf("expiredCalls = %d, want 1", runner.expiredCalls)
	}
	if store.clearCalls != 1 {
		t.Fatalf("clearCalls = %d, want 1", store.clearCalls)
	}
	if s.HasPending() {
		t.Fatal("expected no pending schedule after expired restore")
	}
}

func TestSchedulerCancelKeepsPendingWhenClearFails(t *testing.T) {
	store := &fakeStore{clearErr: errors.New("boom")}
	s := New(store, &fakeRunner{})
	start := time.Date(2026, time.March, 29, 21, 0, 0, 0, time.UTC)

	if err := s.Schedule(Plan{StartTime: start}); err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	if err := s.Cancel(); err == nil {
		t.Fatal("Cancel() error = nil, want non-nil")
	}
	if !s.HasPending() {
		t.Fatal("expected pending schedule to remain when clear fails")
	}
}

func TestSchedulerTickClearsPendingAfterStartEvenWhenStoreClearFails(t *testing.T) {
	store := &fakeStore{clearErr: errors.New("boom")}
	runner := &fakeRunner{}
	s := New(store, runner)
	start := time.Date(2026, time.March, 29, 21, 0, 0, 0, time.UTC)

	if err := s.Schedule(Plan{StartTime: start}); err != nil {
		t.Fatalf("Schedule() error = %v", err)
	}
	if err := s.Tick(start); err == nil {
		t.Fatal("Tick() error = nil, want non-nil")
	}
	if runner.startCalls != 1 {
		t.Fatalf("startCalls = %d, want 1", runner.startCalls)
	}
	if s.HasPending() {
		t.Fatal("expected pending schedule to be cleared after successful start")
	}
}

func TestSchedulerRestoreKeepsSchedulePersistedWhenExpiredClearFails(t *testing.T) {
	store := &fakeStore{
		restored: &Plan{StartTime: time.Date(2026, time.March, 29, 21, 0, 0, 0, time.UTC)},
		clearErr: errors.New("boom"),
	}
	runner := &fakeRunner{}
	s := New(store, runner)

	if err := s.Restore(time.Date(2026, time.March, 29, 21, 1, 0, 0, time.UTC)); err == nil {
		t.Fatal("Restore() error = nil, want non-nil")
	}
	if runner.expiredCalls != 1 {
		t.Fatalf("expiredCalls = %d, want 1", runner.expiredCalls)
	}
	if s.HasPending() {
		t.Fatal("expected no pending schedule after expired restore path")
	}
}

type fakeStore struct {
	saved      *Plan
	restored   *Plan
	saveErr    error
	restoreErr error
	clearErr   error
	clearCalls int
}

func (f *fakeStore) Save(plan Plan) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = &plan
	return nil
}

func (f *fakeStore) Restore() (*Plan, error) {
	if f.restoreErr != nil {
		return nil, f.restoreErr
	}
	if f.restored == nil {
		return nil, nil
	}
	plan := *f.restored
	return &plan, nil
}

func (f *fakeStore) Clear() error {
	f.clearCalls++
	return f.clearErr
}

type fakeRunner struct {
	startErr     error
	expiredErr   error
	startCalls   int
	expiredCalls int
}

func (f *fakeRunner) StartNow() error {
	f.startCalls++
	return f.startErr
}

func (f *fakeRunner) HandleExpiredSchedule() error {
	f.expiredCalls++
	return f.expiredErr
}
