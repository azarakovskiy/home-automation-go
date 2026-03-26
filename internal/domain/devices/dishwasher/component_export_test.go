package dishwasher

import (
	"sync"

	domainnotifications "home-go/internal/domain/notifications"
	"home-go/internal/domain/optimizer"
	"home-go/internal/domain/scheduler"

	ga "saml.dev/gome-assistant"
)

type testController struct{}

func (testController) InitializeModeForScheduled(string) error { return nil }
func (testController) StartDishwasher() error                 { return nil }

// NewTestDishwasher constructs a minimal component for tests in the external package.
func NewTestDishwasher(sm ScheduleStateStore) *Dishwasher {
	return &Dishwasher{
		controller:   testController{},
		optimizer:    optimizer.NewOptimizer(),
		stateManager: sm,
	}
}

// SetNotificationSenderForTest injects a notification sender for tests.
func (d *Dishwasher) SetNotificationSenderForTest(sender NotificationSender) {
	d.notificationService = sender
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
func (d *Dishwasher) HandleScheduleRequestForTest(request scheduler.ScheduleRequest) {
	d.handleScheduleRequest(nil, nil, request)
}

// TestNotificationService captures notifications for assertions.
type TestNotificationService struct {
	mu     sync.Mutex
	events []domainnotifications.Event
	Err    error
}

// NewTestNotificationService returns a notification recorder for tests.
func NewTestNotificationService() *TestNotificationService {
	return &TestNotificationService{}
}

// Notify records the event for later assertions.
func (t *TestNotificationService) Notify(event domainnotifications.Event) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
	return t.Err
}

// Events returns a copy of all recorded events.
func (t *TestNotificationService) Events() []domainnotifications.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]domainnotifications.Event, len(t.events))
	copy(out, t.events)
	return out
}

// LastEvent returns the last recorded event, if any.
func (t *TestNotificationService) LastEvent() (domainnotifications.Event, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.events) == 0 {
		return domainnotifications.Event{}, false
	}
	return t.events[len(t.events)-1], true
}
