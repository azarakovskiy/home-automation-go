package dishwasher

import (
	"home-go/optimization/optimizer"
	"home-go/optimization/scheduled"

	ga "saml.dev/gome-assistant"
)

// NewTestDishwasher constructs a minimal component for tests in the external package.
func NewTestDishwasher(sm ScheduleStateStore) *Dishwasher {
	return &Dishwasher{
		controller:   &Controller{},
		optimizer:    optimizer.NewOptimizer(),
		stateManager: sm,
	}
}

// SetPendingScheduleForTest seeds a pending schedule for test assertions.
func (d *Dishwasher) SetPendingScheduleForTest(schedule *PendingSchedule) {
	d.pendingSchedule = schedule
}

// PendingScheduleForTest exposes the current pending schedule.
func (d *Dishwasher) PendingScheduleForTest() *PendingSchedule {
	return d.pendingSchedule
}

// HandleScheduleFlagChangeForTest triggers the flag change handler.
func (d *Dishwasher) HandleScheduleFlagChangeForTest(data ga.EntityData) {
	d.handleScheduleFlagChange(nil, nil, data)
}

// HandleScheduleRequestForTest triggers the schedule request handler.
func (d *Dishwasher) HandleScheduleRequestForTest(request scheduled.ScheduleRequest) {
	d.handleScheduleRequest(nil, nil, request)
}
