package dishwasher_test

import (
	"testing"
	"time"

	dishwasher_mocks "home-go/mocks/optimization/scheduled/dishwasher"
	"home-go/optimization/optimizer"
	"home-go/optimization/scheduled"
	dishwasher "home-go/optimization/scheduled/dishwasher"

	"go.uber.org/mock/gomock"
	ga "saml.dev/gome-assistant"
)

func TestHandleScheduleFlagChangeCancelsPendingSchedule(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm := dishwasher_mocks.NewMockScheduleStateStore(ctrl)
	d := dishwasher.NewTestDishwasher(sm)
	d.SetPendingScheduleForTest(&dishwasher.PendingSchedule{
		Mode:      dishwasher.ModeAuto,
		StartTime: time.Now().Add(10 * time.Minute),
		Result:    &optimizer.OptimizationResult{},
	})

	sm.EXPECT().ClearSchedule().Return(nil)

	d.HandleScheduleFlagChangeForTest(ga.EntityData{FromState: "on", ToState: "off"})

	if d.PendingScheduleForTest() != nil {
		t.Fatal("expected pending schedule to be cleared")
	}
}

func TestHandleScheduleRequestCancel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm := dishwasher_mocks.NewMockScheduleStateStore(ctrl)
	d := dishwasher.NewTestDishwasher(sm)
	d.SetPendingScheduleForTest(&dishwasher.PendingSchedule{
		Mode:      dishwasher.ModeAuto,
		StartTime: time.Now().Add(30 * time.Minute),
		Result:    &optimizer.OptimizationResult{},
	})

	sm.EXPECT().ClearSchedule().Return(nil)

	d.HandleScheduleRequestForTest(scheduled.ScheduleRequest{
		Device: "dishwasher",
		Mode:   string(dishwasher.ModeCancel),
	})

	if d.PendingScheduleForTest() != nil {
		t.Fatal("expected pending schedule to be cleared by cancel request")
	}
}
