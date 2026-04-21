package dishwasher

import (
	"sync"

	"home-go/internal/domain/optimizer"
	domainscheduler "home-go/internal/domain/scheduler"
	hanotifications "home-go/internal/tech/homeassistant/notifications"
)

type testController struct{}

func (testController) InitializeModeForScheduled(string) error { return nil }
func (testController) StartDishwasher() error                  { return nil }

// NewTestDishwasher constructs a minimal component for tests in the external package.
func NewTestDishwasher(store domainscheduler.Store) *Dishwasher {
	if store == nil {
		store = &TestScheduleStore{}
	}

	return &Dishwasher{
		controller: testController{},
		optimizer:  optimizer.NewOptimizer(),
		scheduler:  domainscheduler.New(store, &testSchedulerRunner{}),
	}
}

// SetNotificationSenderForTest injects a notification sender for tests.
func (d *Dishwasher) SetNotificationSenderForTest(sender NotificationSender) {
	d.notificationService = sender
}

// SetPendingScheduleForTest seeds a pending schedule for test assertions.
func (d *Dishwasher) SetPendingScheduleForTest(schedule *PendingSchedule) {
	if schedule == nil {
		d.scheduler = domainscheduler.New(&TestScheduleStore{}, &testSchedulerRunner{})
		return
	}

	_ = d.scheduler.Schedule(*schedule)
}

// PendingScheduleForTest exposes the current pending schedule.
func (d *Dishwasher) PendingScheduleForTest() *PendingSchedule {
	return d.scheduler.Pending()
}

// CancelPendingScheduleFromDashboardForTest triggers the dashboard cancellation path.
func (d *Dishwasher) CancelPendingScheduleFromDashboardForTest() {
	d.CancelPendingScheduleFromDashboard()
}

// HandleScheduleRequestForTest triggers the schedule request handler.
func (d *Dishwasher) HandleScheduleRequestForTest(request ScheduleRequest) {
	d.handleScheduleRequest(nil, nil, request)
}

type TestScheduleStore struct {
	Saved       *domainscheduler.Plan
	RestorePlan *domainscheduler.Plan
	SaveErr     error
	ClearErr    error
	ClearCalls  int
}

func (t *TestScheduleStore) Save(plan domainscheduler.Plan) error {
	if t.SaveErr != nil {
		return t.SaveErr
	}
	t.Saved = &plan
	return nil
}

func (t *TestScheduleStore) Restore() (*domainscheduler.Plan, error) {
	if t.RestorePlan == nil {
		return nil, nil
	}

	plan := *t.RestorePlan
	return &plan, nil
}

func (t *TestScheduleStore) Clear() error {
	t.ClearCalls++
	return t.ClearErr
}

type testSchedulerRunner struct {
	startErr   error
	expiredErr error
}

func (t *testSchedulerRunner) StartNow() error {
	return t.startErr
}

func (t *testSchedulerRunner) HandleExpiredSchedule() error {
	return t.expiredErr
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
