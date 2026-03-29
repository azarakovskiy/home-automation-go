package dishwasher

import (
	"sync"

	"home-go/internal/domain/optimizer"
	"home-go/internal/domain/scheduler"
	hanotifications "home-go/internal/tech/homeassistant/notifications"
)

type testController struct{}

func (testController) InitializeModeForScheduled(string) error { return nil }
func (testController) StartDishwasher() error                  { return nil }

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

// CancelPendingScheduleFromDashboardForTest triggers the dashboard cancellation path.
func (d *Dishwasher) CancelPendingScheduleFromDashboardForTest() {
	d.CancelPendingScheduleFromDashboard()
}

// HandleScheduleRequestForTest triggers the schedule request handler.
func (d *Dishwasher) HandleScheduleRequestForTest(request scheduler.ScheduleRequest) {
	d.handleScheduleRequest(nil, nil, request)
}

// TestNotificationService captures notifications for assertions.
type TestNotificationService struct {
	mu     sync.Mutex
	events []hanotifications.Event
	Err    error
}

// NewTestNotificationService returns a notification recorder for tests.
func NewTestNotificationService() *TestNotificationService {
	return &TestNotificationService{}
}

// Notify records the event for later assertions.
func (t *TestNotificationService) Notify(event hanotifications.Event) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
	return t.Err
}

// Events returns a copy of all recorded events.
func (t *TestNotificationService) Events() []hanotifications.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]hanotifications.Event, len(t.events))
	copy(out, t.events)
	return out
}

// LastEvent returns the last recorded event, if any.
func (t *TestNotificationService) LastEvent() (hanotifications.Event, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.events) == 0 {
		return hanotifications.Event{}, false
	}
	return t.events[len(t.events)-1], true
}
