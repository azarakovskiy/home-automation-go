package reminders

import (
	"context"
	"testing"
	"time"

	domainreminders "home-go/internal/domain/reminders"
	"home-go/internal/tech/homeassistant/component"
	"home-go/internal/tech/homeassistant/entities"
)

// --- fakes ---

type fakeSwitchHandle struct {
	onCalled        bool
	onCommandCalled bool
	handler         func(context.Context, bool) error
}

func (f *fakeSwitchHandle) On(_ context.Context) error {
	f.onCalled = true
	return nil
}

func (f *fakeSwitchHandle) OnCommand(fn func(context.Context, bool) error) error {
	f.onCommandCalled = true
	f.handler = fn
	return nil
}

type fakeProjector struct {
	switches  map[string]*fakeSwitchHandle
	removed   []string
	reconcile [][]string
	switchErr error
}

func newFakeProjector() *fakeProjector {
	return &fakeProjector{
		switches: make(map[string]*fakeSwitchHandle),
	}
}

func (p *fakeProjector) Switch(_ context.Context, spec entities.SwitchSpec) (switchHandle, error) {
	if p.switchErr != nil {
		return nil, p.switchErr
	}
	h, ok := p.switches[spec.Key]
	if !ok {
		h = &fakeSwitchHandle{}
		p.switches[spec.Key] = h
	}
	return h, nil
}

func (p *fakeProjector) Remove(_ context.Context, key string) error {
	p.removed = append(p.removed, key)
	return nil
}

func (p *fakeProjector) Reconcile(_ context.Context, keep []string) error {
	p.reconcile = append(p.reconcile, keep)
	return nil
}

type fakeManager struct {
	createAction domainreminders.Action
	createErr    error
	ackAction    domainreminders.Action
	ackErr       error
	deleteAction domainreminders.Action
	deleteErr    error
	restoreList  []domainreminders.Reminder
	restoreErr   error
	tickActions  []domainreminders.Action
	tickErr      error

	lastCreateID  domainreminders.ReminderID
	lastAckID     domainreminders.ReminderID
	lastAckUserID string
	lastDeleteID  domainreminders.ReminderID
}

func (m *fakeManager) Create(_ context.Context, id domainreminders.ReminderID, _ []string, _ domainreminders.Schedule, _ domainreminders.DeliveryPolicy, _ domainreminders.Metadata) (domainreminders.Action, error) {
	m.lastCreateID = id
	return m.createAction, m.createErr
}

func (m *fakeManager) Ack(_ context.Context, reminderID domainreminders.ReminderID, userID string) (domainreminders.Action, error) {
	m.lastAckID = reminderID
	m.lastAckUserID = userID
	return m.ackAction, m.ackErr
}

func (m *fakeManager) Delete(_ context.Context, reminderID domainreminders.ReminderID) (domainreminders.Action, error) {
	m.lastDeleteID = reminderID
	return m.deleteAction, m.deleteErr
}

func (m *fakeManager) Restore(_ context.Context) ([]domainreminders.Reminder, error) {
	return m.restoreList, m.restoreErr
}

func (m *fakeManager) Tick(_ context.Context, _ time.Time) ([]domainreminders.Action, error) {
	return m.tickActions, m.tickErr
}

func newTestComponent(proj *fakeProjector, mgr *fakeManager) *Component {
	return &Component{
		Base:    component.Base{},
		runtime: proj,
		manager: mgr,
	}
}

// --- helpers ---

var baseTime = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func reminder1Target(id string) domainreminders.Reminder {
	return domainreminders.Reminder{
		ID:      id,
		Targets: []string{"alice"},
		State:   domainreminders.State{Status: domainreminders.StatusActive},
		Meta:    domainreminders.Metadata{Message: "test reminder"},
	}
}

func reminder2Targets(id string) domainreminders.Reminder {
	rem := reminder1Target(id)
	rem.Targets = []string{"alice", "bob"}
	return rem
}

// --- tests: projectionKey ---

func TestProjectionKey_SlashSanitized(t *testing.T) {
	key := projectionKey("rem/1", "user/alice")
	want := "reminder_rem_1_user_alice"
	if key != want {
		t.Errorf("projectionKey = %q, want %q", key, want)
	}
}

func TestProjectionKey_NoSlash(t *testing.T) {
	key := projectionKey("rem-1", "alice")
	want := "reminder_rem-1_alice"
	if key != want {
		t.Errorf("projectionKey = %q, want %q", key, want)
	}
}

// --- tests: showProjection / removeProjection ---

func TestShowProjection_CreatesSwitchPerTarget(t *testing.T) {
	proj := newFakeProjector()
	c := newTestComponent(proj, &fakeManager{})
	rem := reminder2Targets("rem-1")

	c.showProjection(context.Background(), rem)

	for _, userID := range rem.Targets {
		key := projectionKey(rem.ID, userID)
		sw, ok := proj.switches[key]
		if !ok {
			t.Errorf("switch %q not created", key)
			continue
		}
		if !sw.onCalled {
			t.Errorf("switch %q not set to ON", key)
		}
		if !sw.onCommandCalled {
			t.Errorf("switch %q command handler not registered", key)
		}
	}
}

func TestRemoveProjection_RemovesSwitchPerTarget(t *testing.T) {
	proj := newFakeProjector()
	c := newTestComponent(proj, &fakeManager{})
	rem := reminder2Targets("rem-1")

	c.removeProjection(context.Background(), rem)

	if len(proj.removed) != 2 {
		t.Fatalf("removed %d keys, want 2", len(proj.removed))
	}
	wantKeys := map[string]bool{
		projectionKey("rem-1", "alice"): true,
		projectionKey("rem-1", "bob"):   true,
	}
	for _, k := range proj.removed {
		if !wantKeys[k] {
			t.Errorf("unexpected removed key %q", k)
		}
	}
}

// --- tests: handleCreate ---

func TestHandleCreate_ShowProjectionOnSuccess(t *testing.T) {
	rem := reminder1Target("rem-create")
	proj := newFakeProjector()
	mgr := &fakeManager{
		createAction: domainreminders.Action{Kind: domainreminders.ActionShowProjection, Reminder: rem},
	}
	c := newTestComponent(proj, mgr)

	c.handleCreate(nil, nil, CreateReminderEvent{
		ID:               "rem-create",
		Targets:          []string{"alice"},
		Message:          "take your meds",
		Owner:            "alice",
		Source:           "test",
		TriggerAt:        baseTime.Format(time.RFC3339),
		RequiresAck:      true,
		CompletionPolicy: "all_targets_ack",
		Profile:          "normal",
	})

	if mgr.lastCreateID != "rem-create" {
		t.Errorf("lastCreateID = %q, want %q", mgr.lastCreateID, "rem-create")
	}
	key := projectionKey("rem-create", "alice")
	if _, ok := proj.switches[key]; !ok {
		t.Errorf("switch %q not created after create event", key)
	}
}

func TestHandleCreate_InvalidTriggerAt_NoProjection(t *testing.T) {
	proj := newFakeProjector()
	mgr := &fakeManager{}
	c := newTestComponent(proj, mgr)

	// Should not panic; should log and return
	c.handleCreate(nil, nil, CreateReminderEvent{
		ID:        "bad",
		TriggerAt: "not-a-time",
	})

	if mgr.lastCreateID != "" {
		t.Errorf("manager.Create should not have been called")
	}
	if len(proj.switches) != 0 {
		t.Errorf("no switches should be created on parse error")
	}
}

func TestHandleCreate_RecurringSchedule(t *testing.T) {
	rem := reminder1Target("rem-recur")
	proj := newFakeProjector()
	mgr := &fakeManager{
		createAction: domainreminders.Action{Kind: domainreminders.ActionShowProjection, Reminder: rem},
	}
	c := newTestComponent(proj, mgr)

	c.handleCreate(nil, nil, CreateReminderEvent{
		ID:               "rem-recur",
		Targets:          []string{"alice"},
		TriggerAt:        baseTime.Format(time.RFC3339),
		RecurEvery:       "1h",
		CompletionPolicy: "any_target_ack",
		Profile:          "quiet",
	})

	if mgr.lastCreateID != "rem-recur" {
		t.Errorf("lastCreateID = %q, want %q", mgr.lastCreateID, "rem-recur")
	}
}

func TestHandleCreate_InvalidRecurEvery_NoProjection(t *testing.T) {
	proj := newFakeProjector()
	mgr := &fakeManager{}
	c := newTestComponent(proj, mgr)

	c.handleCreate(nil, nil, CreateReminderEvent{
		ID:         "bad",
		TriggerAt:  baseTime.Format(time.RFC3339),
		RecurEvery: "not-a-duration",
	})

	if mgr.lastCreateID != "" {
		t.Errorf("manager.Create should not have been called")
	}
}

// --- tests: handleAck ---

func TestHandleAck_ShowProjection(t *testing.T) {
	rem := reminder2Targets("rem-ack")
	proj := newFakeProjector()
	mgr := &fakeManager{
		ackAction: domainreminders.Action{Kind: domainreminders.ActionShowProjection, Reminder: rem},
	}
	c := newTestComponent(proj, mgr)

	c.handleAck(nil, nil, AckReminderEvent{ID: "rem-ack", UserID: "alice"})

	if mgr.lastAckID != "rem-ack" {
		t.Errorf("lastAckID = %q, want %q", mgr.lastAckID, "rem-ack")
	}
	if mgr.lastAckUserID != "alice" {
		t.Errorf("lastAckUserID = %q, want %q", mgr.lastAckUserID, "alice")
	}
	// ShowProjection creates switches for all targets
	for _, uid := range rem.Targets {
		key := projectionKey("rem-ack", uid)
		if _, ok := proj.switches[key]; !ok {
			t.Errorf("switch %q not created after ack", key)
		}
	}
}

func TestHandleAck_RemoveProjection(t *testing.T) {
	rem := reminder1Target("rem-done")
	proj := newFakeProjector()
	mgr := &fakeManager{
		ackAction: domainreminders.Action{Kind: domainreminders.ActionRemoveProjection, Reminder: rem},
	}
	c := newTestComponent(proj, mgr)

	c.handleAck(nil, nil, AckReminderEvent{ID: "rem-done", UserID: "alice"})

	key := projectionKey("rem-done", "alice")
	if len(proj.removed) == 0 || proj.removed[0] != key {
		t.Errorf("switch %q not removed after complete ack", key)
	}
}

// --- tests: handleDelete ---

func TestHandleDelete_RemovesProjection(t *testing.T) {
	rem := reminder2Targets("rem-del")
	proj := newFakeProjector()
	mgr := &fakeManager{
		deleteAction: domainreminders.Action{Kind: domainreminders.ActionRemoveProjection, Reminder: rem},
	}
	c := newTestComponent(proj, mgr)

	c.handleDelete(nil, nil, DeleteReminderEvent{ID: "rem-del"})

	if mgr.lastDeleteID != "rem-del" {
		t.Errorf("lastDeleteID = %q, want %q", mgr.lastDeleteID, "rem-del")
	}
	if len(proj.removed) != 2 {
		t.Errorf("removed %d keys, want 2", len(proj.removed))
	}
}

// --- tests: tick ---

func TestTick_AppliesAllActions(t *testing.T) {
	remShow := reminder1Target("rem-show")
	remRemove := reminder1Target("rem-remove")

	proj := newFakeProjector()
	mgr := &fakeManager{
		tickActions: []domainreminders.Action{
			{Kind: domainreminders.ActionShowProjection, Reminder: remShow},
			{Kind: domainreminders.ActionRemoveProjection, Reminder: remRemove},
		},
	}
	c := newTestComponent(proj, mgr)
	c.tick(nil, nil)

	// Show was applied
	showKey := projectionKey("rem-show", "alice")
	if _, ok := proj.switches[showKey]; !ok {
		t.Errorf("switch %q not created from tick ShowProjection", showKey)
	}
	// Remove was applied
	removeKey := projectionKey("rem-remove", "alice")
	if len(proj.removed) == 0 || proj.removed[0] != removeKey {
		t.Errorf("switch %q not removed from tick RemoveProjection", removeKey)
	}
}

func TestTick_NoopActions_NoProjectionChanges(t *testing.T) {
	proj := newFakeProjector()
	mgr := &fakeManager{
		tickActions: []domainreminders.Action{
			{Kind: domainreminders.ActionNoop},
		},
	}
	c := newTestComponent(proj, mgr)
	c.tick(nil, nil)

	if len(proj.switches) != 0 {
		t.Errorf("no switches should be created for noop actions")
	}
	if len(proj.removed) != 0 {
		t.Errorf("no removes should happen for noop actions")
	}
}

// --- tests: Restore ---

func TestRestore_ShowsProjectionsAndReconciles(t *testing.T) {
	rem1 := reminder1Target("rem-1")
	rem2 := reminder2Targets("rem-2")

	proj := newFakeProjector()
	mgr := &fakeManager{
		restoreList: []domainreminders.Reminder{rem1, rem2},
	}
	c := newTestComponent(proj, mgr)

	if err := c.Restore(context.Background()); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Switches created for all targets
	for _, rem := range []domainreminders.Reminder{rem1, rem2} {
		for _, uid := range rem.Targets {
			key := projectionKey(rem.ID, uid)
			if _, ok := proj.switches[key]; !ok {
				t.Errorf("switch %q not created on restore", key)
			}
		}
	}

	// Reconcile called once with all expected keys
	if len(proj.reconcile) != 1 {
		t.Fatalf("reconcile called %d times, want 1", len(proj.reconcile))
	}
	reconcileKeys := proj.reconcile[0]
	wantKeys := map[string]bool{
		projectionKey("rem-1", "alice"): true,
		projectionKey("rem-2", "alice"): true,
		projectionKey("rem-2", "bob"):   true,
	}
	if len(reconcileKeys) != len(wantKeys) {
		t.Errorf("reconcile keys = %v, want %v", reconcileKeys, wantKeys)
	}
	for _, k := range reconcileKeys {
		if !wantKeys[k] {
			t.Errorf("unexpected reconcile key %q", k)
		}
	}
}

func TestRestore_EmptyList_StillReconciles(t *testing.T) {
	proj := newFakeProjector()
	mgr := &fakeManager{restoreList: nil}
	c := newTestComponent(proj, mgr)

	if err := c.Restore(context.Background()); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	if len(proj.reconcile) != 1 {
		t.Fatalf("reconcile called %d times, want 1", len(proj.reconcile))
	}
}

// --- tests: MQTT ack command handler ---

func TestAckCommandHandler_OffAcksReminder(t *testing.T) {
	rem := reminder1Target("rem-ack-cmd")
	proj := newFakeProjector()
	mgr := &fakeManager{
		ackAction: domainreminders.Action{Kind: domainreminders.ActionRemoveProjection, Reminder: rem},
	}
	c := newTestComponent(proj, mgr)

	// showProjection registers the command handler
	c.showProjection(context.Background(), rem)

	key := projectionKey("rem-ack-cmd", "alice")
	sw, ok := proj.switches[key]
	if !ok {
		t.Fatalf("switch %q not created", key)
	}
	if sw.handler == nil {
		t.Fatal("command handler not registered")
	}

	// Simulate user pressing OFF (acknowledgement)
	if err := sw.handler(context.Background(), false); err != nil {
		t.Fatalf("command handler error = %v", err)
	}

	if mgr.lastAckID != "rem-ack-cmd" {
		t.Errorf("lastAckID = %q, want %q", mgr.lastAckID, "rem-ack-cmd")
	}
	if mgr.lastAckUserID != "alice" {
		t.Errorf("lastAckUserID = %q, want %q", mgr.lastAckUserID, "alice")
	}
	// RemoveProjection removes the switch
	if len(proj.removed) == 0 || proj.removed[0] != key {
		t.Errorf("switch %q not removed after ack command", key)
	}
}

func TestAckCommandHandler_OnCommandIgnored(t *testing.T) {
	rem := reminder1Target("rem-on-cmd")
	proj := newFakeProjector()
	mgr := &fakeManager{
		ackAction: domainreminders.Action{Kind: domainreminders.ActionShowProjection, Reminder: rem},
	}
	c := newTestComponent(proj, mgr)
	c.showProjection(context.Background(), rem)

	key := projectionKey("rem-on-cmd", "alice")
	sw := proj.switches[key]

	// Reset onCalled to track the ON re-set inside the handler
	sw.onCalled = false

	// Simulate ON command (should just set back to ON, not ack)
	if err := sw.handler(context.Background(), true); err != nil {
		t.Fatalf("command handler error = %v", err)
	}

	if !sw.onCalled {
		t.Error("expected ON command to call sw.On() to re-affirm the state")
	}

	// Manager.Ack should NOT have been called
	if mgr.lastAckID != "" {
		t.Errorf("Ack should not be called for ON command")
	}
}

// --- tests: component interface ---

func TestEventListeners_ReturnsThree(t *testing.T) {
	c := newTestComponent(newFakeProjector(), &fakeManager{})
	listeners := c.EventListeners()
	if len(listeners) != 3 {
		t.Errorf("EventListeners() = %d listeners, want 3", len(listeners))
	}
}

func TestIntervals_ReturnsOne(t *testing.T) {
	c := newTestComponent(newFakeProjector(), &fakeManager{})
	intervals := c.Intervals()
	if len(intervals) != 1 {
		t.Errorf("Intervals() = %d intervals, want 1", len(intervals))
	}
}
