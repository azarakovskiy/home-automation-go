package dishwasher

import (
	"context"
	"errors"
	"testing"
	"time"

	domaindishwasher "home-go/internal/domain/devices/dishwasher"
	"home-go/internal/domain/optimizer"
	"home-go/internal/mocks"
	"home-go/internal/tech/homeassistant/entities"

	"go.uber.org/mock/gomock"
	ga "saml.dev/gome-assistant"
)

func TestNewStateManagerRequiresRuntime(t *testing.T) {
	sm, err := NewStateManager(nil, nil, nil)
	if err == nil {
		t.Fatal("NewStateManager() error = nil, want error")
	}
	if sm != nil {
		t.Fatal("NewStateManager() manager = non-nil, want nil")
	}
}

func TestStateManagerSaveSchedule(t *testing.T) {
	startTime := time.Date(2026, time.March, 29, 21, 15, 0, 0, time.UTC)

	sm := &StateManager{
		scheduled:      &fakeScheduleSwitch{entityID: "switch.kitchen_dishwasher_is_scheduled"},
		mode:           &fakeTextState{entityID: "sensor.kitchen_dishwasher_scheduled_mode"},
		startTime:      &fakeTextState{entityID: "sensor.kitchen_dishwasher_scheduled_start"},
		estimatedCost:  &fakeNumberState{entityID: "sensor.kitchen_dishwasher_estimated_cost"},
		currentCost:    &fakeNumberState{entityID: "sensor.kitchen_dishwasher_current_cost"},
		savingsPercent: &fakeNumberState{entityID: "sensor.kitchen_dishwasher_savings_percent"},
	}

	err := sm.SaveSchedule(&domaindishwasher.PendingSchedule{
		Mode:      domaindishwasher.ModeAuto,
		StartTime: startTime,
		Result: &optimizer.OptimizationResult{
			EstimatedCost:  0.50,
			CurrentCost:    0.60,
			SavingsPercent: 16.67,
		},
	})
	if err != nil {
		t.Fatalf("SaveSchedule() error = %v", err)
	}

	if !sm.scheduled.(*fakeScheduleSwitch).onCalled {
		t.Fatal("scheduled.On() was not called")
	}
	if got := sm.mode.(*fakeTextState).lastValue; got != "auto" {
		t.Fatalf("mode = %q, want %q", got, "auto")
	}
	if got := sm.startTime.(*fakeTextState).lastValue; got != startTime.Format(time.RFC3339) {
		t.Fatalf("startTime = %q, want %q", got, startTime.Format(time.RFC3339))
	}
	if got := sm.estimatedCost.(*fakeNumberState).lastValue; got != 0.50 {
		t.Fatalf("estimatedCost = %f, want %f", got, 0.50)
	}
	if got := sm.currentCost.(*fakeNumberState).lastValue; got != 0.60 {
		t.Fatalf("currentCost = %f, want %f", got, 0.60)
	}
	if got := sm.savingsPercent.(*fakeNumberState).lastValue; got != 16.67 {
		t.Fatalf("savingsPercent = %f, want %f", got, 16.67)
	}
}

func TestStateManagerSaveScheduleRequiresScheduledFlag(t *testing.T) {
	sm := &StateManager{
		scheduled:      &fakeScheduleSwitch{entityID: "switch.kitchen_dishwasher_is_scheduled", onErr: errors.New("boom")},
		mode:           &fakeTextState{entityID: "sensor.kitchen_dishwasher_scheduled_mode"},
		startTime:      &fakeTextState{entityID: "sensor.kitchen_dishwasher_scheduled_start"},
		estimatedCost:  &fakeNumberState{entityID: "sensor.kitchen_dishwasher_estimated_cost"},
		currentCost:    &fakeNumberState{entityID: "sensor.kitchen_dishwasher_current_cost"},
		savingsPercent: &fakeNumberState{entityID: "sensor.kitchen_dishwasher_savings_percent"},
	}

	err := sm.SaveSchedule(&domaindishwasher.PendingSchedule{
		Mode:      domaindishwasher.ModeAuto,
		StartTime: time.Now().Add(time.Hour),
		Result:    &optimizer.OptimizationResult{},
	})
	if err == nil {
		t.Fatal("SaveSchedule() error = nil, want error")
	}
}

func TestStateManagerRestoreSchedule(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	futureTime := time.Now().Add(time.Hour).Round(time.Second)
	mockState := mocks.NewMockState(ctrl)
	mockState.EXPECT().Get("switch.kitchen_dishwasher_is_scheduled").Return(ga.EntityState{State: "on"}, nil)
	mockState.EXPECT().Get("sensor.kitchen_dishwasher_scheduled_mode").Return(ga.EntityState{State: "auto"}, nil)
	mockState.EXPECT().Get("sensor.kitchen_dishwasher_scheduled_start").Return(ga.EntityState{State: futureTime.Format(time.RFC3339)}, nil)
	mockState.EXPECT().Get("sensor.kitchen_dishwasher_estimated_cost").Return(ga.EntityState{State: "0.50"}, nil)
	mockState.EXPECT().Get("sensor.kitchen_dishwasher_current_cost").Return(ga.EntityState{State: "0.60"}, nil)
	mockState.EXPECT().Get("sensor.kitchen_dishwasher_savings_percent").Return(ga.EntityState{State: "16.67"}, nil)

	sm := &StateManager{
		state:          mockState,
		scheduled:      &fakeScheduleSwitch{entityID: "switch.kitchen_dishwasher_is_scheduled"},
		mode:           &fakeTextState{entityID: "sensor.kitchen_dishwasher_scheduled_mode"},
		startTime:      &fakeTextState{entityID: "sensor.kitchen_dishwasher_scheduled_start"},
		estimatedCost:  &fakeNumberState{entityID: "sensor.kitchen_dishwasher_estimated_cost"},
		currentCost:    &fakeNumberState{entityID: "sensor.kitchen_dishwasher_current_cost"},
		savingsPercent: &fakeNumberState{entityID: "sensor.kitchen_dishwasher_savings_percent"},
	}

	schedule, err := sm.RestoreSchedule()
	if err != nil {
		t.Fatalf("RestoreSchedule() error = %v", err)
	}
	if schedule == nil {
		t.Fatal("RestoreSchedule() schedule = nil, want non-nil")
	}
	if schedule.Mode != domaindishwasher.ModeAuto {
		t.Fatalf("Mode = %s, want %s", schedule.Mode, domaindishwasher.ModeAuto)
	}
	if !schedule.StartTime.Equal(futureTime) {
		t.Fatalf("StartTime = %s, want %s", schedule.StartTime, futureTime)
	}
	const epsilon = 0.0001
	if diff := schedule.Result.Savings - 0.10; diff < -epsilon || diff > epsilon {
		t.Fatalf("Savings = %.4f, want %.4f", schedule.Result.Savings, 0.10)
	}
}

func TestStateManagerRestoreScheduleTreatsMissingRuntimeEntityAsEmpty(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockState := mocks.NewMockState(ctrl)
	mockState.EXPECT().Get("switch.kitchen_dishwasher_is_scheduled").Return(ga.EntityState{}, errors.New("entity not found"))

	sm := &StateManager{
		state:     mockState,
		scheduled: &fakeScheduleSwitch{entityID: "switch.kitchen_dishwasher_is_scheduled"},
	}

	schedule, err := sm.RestoreSchedule()
	if err != nil {
		t.Fatalf("RestoreSchedule() error = %v, want nil", err)
	}
	if schedule != nil {
		t.Fatalf("RestoreSchedule() schedule = %v, want nil", schedule)
	}
}

func TestStateManagerRestoreScheduleClearsExpiredState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pastTime := time.Now().Add(-time.Hour).Round(time.Second)
	mockState := mocks.NewMockState(ctrl)
	mockState.EXPECT().Get("switch.kitchen_dishwasher_is_scheduled").Return(ga.EntityState{State: "on"}, nil)
	mockState.EXPECT().Get("sensor.kitchen_dishwasher_scheduled_mode").Return(ga.EntityState{State: "auto"}, nil)
	mockState.EXPECT().Get("sensor.kitchen_dishwasher_scheduled_start").Return(ga.EntityState{State: pastTime.Format(time.RFC3339)}, nil)
	mockState.EXPECT().Get("sensor.kitchen_dishwasher_estimated_cost").Return(ga.EntityState{State: "0.50"}, nil)
	mockState.EXPECT().Get("sensor.kitchen_dishwasher_current_cost").Return(ga.EntityState{State: "0.60"}, nil)
	mockState.EXPECT().Get("sensor.kitchen_dishwasher_savings_percent").Return(ga.EntityState{State: "16.67"}, nil)
	mockState.EXPECT().Get(entities.Switch.KitchenDishwasherSocket).Return(ga.EntityState{State: "on"}, nil)

	sm := &StateManager{
		state:          mockState,
		controller:     &Controller{},
		scheduled:      &fakeScheduleSwitch{entityID: "switch.kitchen_dishwasher_is_scheduled"},
		mode:           &fakeTextState{entityID: "sensor.kitchen_dishwasher_scheduled_mode"},
		startTime:      &fakeTextState{entityID: "sensor.kitchen_dishwasher_scheduled_start"},
		estimatedCost:  &fakeNumberState{entityID: "sensor.kitchen_dishwasher_estimated_cost"},
		currentCost:    &fakeNumberState{entityID: "sensor.kitchen_dishwasher_current_cost"},
		savingsPercent: &fakeNumberState{entityID: "sensor.kitchen_dishwasher_savings_percent"},
	}

	schedule, err := sm.RestoreSchedule()
	if err != nil {
		t.Fatalf("RestoreSchedule() error = %v", err)
	}
	if schedule != nil {
		t.Fatalf("RestoreSchedule() schedule = %v, want nil", schedule)
	}
	if !sm.scheduled.(*fakeScheduleSwitch).offCalled {
		t.Fatal("scheduled.Off() was not called when clearing expired state")
	}
}

func TestStateManagerClearSchedule(t *testing.T) {
	sm := &StateManager{
		scheduled:      &fakeScheduleSwitch{entityID: "switch.kitchen_dishwasher_is_scheduled"},
		mode:           &fakeTextState{entityID: "sensor.kitchen_dishwasher_scheduled_mode"},
		startTime:      &fakeTextState{entityID: "sensor.kitchen_dishwasher_scheduled_start"},
		estimatedCost:  &fakeNumberState{entityID: "sensor.kitchen_dishwasher_estimated_cost"},
		currentCost:    &fakeNumberState{entityID: "sensor.kitchen_dishwasher_current_cost"},
		savingsPercent: &fakeNumberState{entityID: "sensor.kitchen_dishwasher_savings_percent"},
	}

	if err := sm.ClearSchedule(); err != nil {
		t.Fatalf("ClearSchedule() error = %v", err)
	}
	if !sm.scheduled.(*fakeScheduleSwitch).offCalled {
		t.Fatal("scheduled.Off() was not called")
	}
	if got := sm.mode.(*fakeTextState).lastValue; got != dishwasherScheduledModeCleared {
		t.Fatalf("mode = %q, want %q", got, dishwasherScheduledModeCleared)
	}
	if got := sm.startTime.(*fakeTextState).lastValue; got != "" {
		t.Fatalf("startTime = %q, want empty string", got)
	}
}

func TestStateManagerIsScheduleCancelled(t *testing.T) {
	tests := []struct {
		name      string
		state     ga.EntityState
		err       error
		want      bool
		wantError bool
	}{
		{
			name:  "active schedule",
			state: ga.EntityState{State: "on"},
		},
		{
			name:  "cancelled schedule",
			state: ga.EntityState{State: "off"},
			want:  true,
		},
		{
			name: "missing schedule entity is treated as not cancelled",
			err:  errors.New("entity not found"),
		},
		{
			name:      "other state errors bubble up",
			err:       errors.New("boom"),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockState := mocks.NewMockState(ctrl)
			mockState.EXPECT().Get("switch.kitchen_dishwasher_is_scheduled").Return(tt.state, tt.err)

			sm := &StateManager{
				state:     mockState,
				scheduled: &fakeScheduleSwitch{entityID: "switch.kitchen_dishwasher_is_scheduled"},
			}

			cancelled, err := sm.IsScheduleCancelled()
			if (err != nil) != tt.wantError {
				t.Fatalf("IsScheduleCancelled() error = %v, wantError %v", err, tt.wantError)
			}
			if !tt.wantError && cancelled != tt.want {
				t.Fatalf("IsScheduleCancelled() = %v, want %v", cancelled, tt.want)
			}
		})
	}
}

func TestStateManagerOnScheduledCommand(t *testing.T) {
	switchHandle := &fakeScheduleSwitch{entityID: "switch.kitchen_dishwasher_is_scheduled"}
	sm := &StateManager{scheduled: switchHandle}

	if err := sm.OnScheduledCommand(func(context.Context, bool) error { return nil }); err != nil {
		t.Fatalf("OnScheduledCommand() error = %v", err)
	}
	if !switchHandle.onCommandCalled {
		t.Fatal("OnCommand() was not delegated to switch handle")
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

type fakeNumberState struct {
	entityID  string
	lastValue float64
	err       error
}

func (f *fakeNumberState) Set(_ context.Context, value float64) error {
	f.lastValue = value
	return f.err
}

func (f *fakeNumberState) EntityID() string {
	return f.entityID
}
