package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	domainscheduler "home-go/internal/domain/scheduler"
	"home-go/internal/mocks"

	"go.uber.org/mock/gomock"
	ga "saml.dev/gome-assistant"
)

func TestStoreSave(t *testing.T) {
	startTime := time.Date(2026, time.March, 29, 21, 15, 0, 0, time.UTC)
	store := newStoreFromHandles(
		nil,
		&fakeScheduleSwitch{entityID: "switch.test_scheduled"},
		&fakeTextState{entityID: "sensor.test_scheduled_at"},
	)

	err := store.Save(domainscheduler.Plan{StartTime: startTime})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if !store.scheduled.(*fakeScheduleSwitch).onCalled {
		t.Fatal("scheduled.On() was not called")
	}
	if got := store.startTime.(*fakeTextState).lastValue; got != startTime.Format(time.RFC3339) {
		t.Fatalf("scheduled time = %q, want %q", got, startTime.Format(time.RFC3339))
	}
}

func TestStoreRestore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	startTime := time.Now().Add(time.Hour).Round(time.Second)
	mockState := mocks.NewMockState(ctrl)
	mockState.EXPECT().Get("switch.test_scheduled").Return(ga.EntityState{State: "on"}, nil)
	mockState.EXPECT().Get("sensor.test_scheduled_at").Return(ga.EntityState{State: startTime.Format(time.RFC3339)}, nil)

	store := newStoreFromHandles(
		mockState,
		&fakeScheduleSwitch{entityID: "switch.test_scheduled"},
		&fakeTextState{entityID: "sensor.test_scheduled_at"},
	)

	plan, err := store.Restore()
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if plan == nil {
		t.Fatal("Restore() = nil, want non-nil")
	}
	if !plan.StartTime.Equal(startTime) {
		t.Fatalf("StartTime = %s, want %s", plan.StartTime, startTime)
	}
}

func TestStoreRestoreReturnsNilWhenSwitchMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockState := mocks.NewMockState(ctrl)
	mockState.EXPECT().Get("switch.test_scheduled").Return(ga.EntityState{}, errors.New("entity not found"))

	store := newStoreFromHandles(
		mockState,
		&fakeScheduleSwitch{entityID: "switch.test_scheduled"},
		&fakeTextState{entityID: "sensor.test_scheduled_at"},
	)

	plan, err := store.Restore()
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if plan != nil {
		t.Fatalf("Restore() = %v, want nil", plan)
	}
}

func TestStoreClear(t *testing.T) {
	store := newStoreFromHandles(
		nil,
		&fakeScheduleSwitch{entityID: "switch.test_scheduled"},
		&fakeTextState{entityID: "sensor.test_scheduled_at"},
	)

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if !store.scheduled.(*fakeScheduleSwitch).offCalled {
		t.Fatal("scheduled.Off() was not called")
	}
	if got := store.startTime.(*fakeTextState).lastValue; got != "" {
		t.Fatalf("scheduled time = %q, want empty string", got)
	}
}

func TestStoreOnCommand(t *testing.T) {
	switchHandle := &fakeScheduleSwitch{entityID: "switch.test_scheduled"}
	store := newStoreFromHandles(nil, switchHandle, &fakeTextState{entityID: "sensor.test_scheduled_at"})

	if err := store.OnCommand(func(context.Context, bool) error { return nil }); err != nil {
		t.Fatalf("OnCommand() error = %v", err)
	}
	if !switchHandle.onCommandCalled {
		t.Fatal("OnCommand() was not delegated")
	}
}

func TestParseScheduledTime(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "rfc3339", value: "2026-03-29T12:30:00Z"},
		{name: "legacy", value: "2026-03-29 12:30:00"},
		{name: "invalid", value: "not-a-time", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseScheduledTime(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseScheduledTime() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

type fakeScheduleSwitch struct {
	entityID        string
	onCalled        bool
	offCalled       bool
	onErr           error
	offErr          error
	onCommandCalled bool
	onCommandErr    error
}

func (f *fakeScheduleSwitch) On(context.Context) error {
	f.onCalled = true
	return f.onErr
}

func (f *fakeScheduleSwitch) Off(context.Context) error {
	f.offCalled = true
	return f.offErr
}

func (f *fakeScheduleSwitch) OnCommand(func(context.Context, bool) error) error {
	f.onCommandCalled = true
	return f.onCommandErr
}

func (f *fakeScheduleSwitch) EntityID() string {
	return f.entityID
}

type fakeTextState struct {
	entityID  string
	lastValue string
	err       error
}

func (f *fakeTextState) Set(_ context.Context, value string) error {
	f.lastValue = value
	return f.err
}

func (f *fakeTextState) EntityID() string {
	return f.entityID
}
