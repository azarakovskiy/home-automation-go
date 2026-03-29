package dishwasher

import (
	"errors"
	"testing"

	"home-go/internal/mocks"
	"home-go/internal/tech/homeassistant/entities"

	"go.uber.org/mock/gomock"
	ga "saml.dev/gome-assistant"
)

func TestSchedulerRunnerHandleExpiredSchedule(t *testing.T) {
	t.Run("does nothing when socket already on", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockState := mocks.NewMockState(ctrl)
		mockState.EXPECT().Get(entities.Switch.KitchenDishwasherSocket).Return(ga.EntityState{State: "on"}, nil)

		started := &fakeScheduleStarter{}
		runner := schedulerRunner{state: mockState, controller: started}

		if err := runner.HandleExpiredSchedule(); err != nil {
			t.Fatalf("HandleExpiredSchedule() error = %v", err)
		}
		if started.calls != 0 {
			t.Fatalf("start calls = %d, want 0", started.calls)
		}
	})

	t.Run("starts dishwasher when socket off", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockState := mocks.NewMockState(ctrl)
		mockState.EXPECT().Get(entities.Switch.KitchenDishwasherSocket).Return(ga.EntityState{State: "off"}, nil)

		started := &fakeScheduleStarter{}
		runner := schedulerRunner{state: mockState, controller: started}

		if err := runner.HandleExpiredSchedule(); err != nil {
			t.Fatalf("HandleExpiredSchedule() error = %v", err)
		}
		if started.calls != 1 {
			t.Fatalf("start calls = %d, want 1", started.calls)
		}
	})

	t.Run("returns socket lookup error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockState := mocks.NewMockState(ctrl)
		mockState.EXPECT().Get(entities.Switch.KitchenDishwasherSocket).Return(ga.EntityState{}, errors.New("boom"))

		runner := schedulerRunner{state: mockState, controller: &fakeScheduleStarter{}}
		if err := runner.HandleExpiredSchedule(); err == nil {
			t.Fatal("HandleExpiredSchedule() error = nil, want non-nil")
		}
	})

	t.Run("returns start error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mockState := mocks.NewMockState(ctrl)
		mockState.EXPECT().Get(entities.Switch.KitchenDishwasherSocket).Return(ga.EntityState{State: "off"}, nil)

		runner := schedulerRunner{
			state:      mockState,
			controller: &fakeScheduleStarter{err: errors.New("boom")},
		}
		if err := runner.HandleExpiredSchedule(); err == nil {
			t.Fatal("HandleExpiredSchedule() error = nil, want non-nil")
		}
	})
}

type fakeScheduleStarter struct {
	calls int
	err   error
}

func (f *fakeScheduleStarter) StartDishwasher() error {
	f.calls++
	return f.err
}
