package reminders_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	domainreminders "home-go/internal/domain/reminders"
	"home-go/internal/tech/homeassistant/component"
	hareminders "home-go/internal/tech/homeassistant/devices/reminders"
	"home-go/internal/tech/homeassistant/entities"

	ga "saml.dev/gome-assistant"
)

// fakeTransport records Subscribe calls for assertions.
type fakeTransport struct {
	subscriptions map[string]func(context.Context, string, []byte)
}

func newFakeTransport() *fakeTransport {
	return &fakeTransport{subscriptions: make(map[string]func(context.Context, string, []byte))}
}

func (f *fakeTransport) Subscribe(_ context.Context, topic string, h func(context.Context, string, []byte)) error {
	f.subscriptions[topic] = h
	return nil
}

func (f *fakeTransport) dispatch(topic string, payload []byte) {
	if h, ok := f.subscriptions[topic]; ok {
		h(context.Background(), topic, payload)
	}
}

// fakeManager records calls for assertions.
type fakeManager struct {
	lastAckID     string
	lastAckUserID string
	lastDeleteID  string
	lastCreate    domainreminders.CreateCommand
	listResult    []domainreminders.Reminder
}

func (f *fakeManager) Create(_ context.Context, cmd domainreminders.CreateCommand) (domainreminders.Reminder, error) {
	f.lastCreate = cmd
	return domainreminders.Reminder{ID: cmd.ID, Meta: domainreminders.Metadata{Message: cmd.Meta.Message}}, nil
}

func (f *fakeManager) Ack(_ context.Context, id, userID string) error {
	f.lastAckID = id
	f.lastAckUserID = userID
	return nil
}

func (f *fakeManager) Delete(_ context.Context, id string) error {
	f.lastDeleteID = id
	return nil
}

func (f *fakeManager) Tick(_ context.Context, _ time.Time) error { return nil }

func (f *fakeManager) List(_ context.Context) ([]domainreminders.Reminder, error) {
	return f.listResult, nil
}

// fakeSwitchHandle records On/Off calls but otherwise no-ops.
type fakeSwitchHandle struct{}

func (f *fakeSwitchHandle) On(_ context.Context) error                          { return nil }
func (f *fakeSwitchHandle) Off(_ context.Context) error                         { return nil }
func (f *fakeSwitchHandle) OnCommand(_ func(context.Context, bool) error) error { return nil }

// fakeEntityRuntime records entity operations.
type fakeEntityRuntime struct {
	switchKeys []string
	removeKeys []string
}

func (f *fakeEntityRuntime) Switch(_ context.Context, spec entities.SwitchSpec) (hareminders.SwitchHandle, error) {
	f.switchKeys = append(f.switchKeys, spec.Key)
	return &fakeSwitchHandle{}, nil
}

func (f *fakeEntityRuntime) Remove(_ context.Context, key string) error {
	f.removeKeys = append(f.removeKeys, key)
	return nil
}

func newTestHandler(t *testing.T, transport *fakeTransport, mgr *fakeManager) *hareminders.Handler {
	t.Helper()
	base := component.Base{Service: (*ga.Service)(nil)}
	return hareminders.New(base, transport, mgr, &fakeEntityRuntime{}, "homeapp")
}

func TestHandler_Start_SubscribesToThreeTopics(t *testing.T) {
	tr := newFakeTransport()
	mgr := &fakeManager{}
	h := newTestHandler(t, tr, mgr)

	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	for _, suffix := range []string{"create", "ack", "delete"} {
		topic := "homeapp/reminders/" + suffix
		if _, ok := tr.subscriptions[topic]; !ok {
			t.Errorf("expected subscription to %s", topic)
		}
	}
}

func TestHandler_Start_DeclaresEntityForEachExistingReminder(t *testing.T) {
	tr := newFakeTransport()
	mgr := &fakeManager{
		listResult: []domainreminders.Reminder{
			{ID: "r1", Meta: domainreminders.Metadata{Message: "take pills"}},
			{ID: "r2", Meta: domainreminders.Metadata{Message: "drink water"}},
		},
	}
	entityRT := &fakeEntityRuntime{}
	base := component.Base{Service: (*ga.Service)(nil)}
	h := hareminders.New(base, tr, mgr, entityRT, "homeapp")

	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if got := len(entityRT.switchKeys); got != 2 {
		t.Errorf("Switch called %d times at startup, want 2", got)
	}
}

func TestHandler_HandleAck_CallsManagerAck(t *testing.T) {
	tr := newFakeTransport()
	mgr := &fakeManager{}
	h := newTestHandler(t, tr, mgr)
	_ = h.Start(context.Background())

	payload, _ := json.Marshal(map[string]string{"id": "r1", "user_id": "u1"})
	tr.dispatch("homeapp/reminders/ack", payload)

	if mgr.lastAckID != "r1" || mgr.lastAckUserID != "u1" {
		t.Errorf("Ack called with id=%q user=%q, want r1/u1", mgr.lastAckID, mgr.lastAckUserID)
	}
}

func TestHandler_HandleDelete_CallsManagerDelete(t *testing.T) {
	tr := newFakeTransport()
	mgr := &fakeManager{}
	h := newTestHandler(t, tr, mgr)
	_ = h.Start(context.Background())

	payload, _ := json.Marshal(map[string]string{"id": "r1"})
	tr.dispatch("homeapp/reminders/delete", payload)

	if mgr.lastDeleteID != "r1" {
		t.Errorf("Delete called with id=%q, want r1", mgr.lastDeleteID)
	}
}

func TestHandler_HandleDelete_RemovesEntityFromRuntime(t *testing.T) {
	tr := newFakeTransport()
	rem := domainreminders.Reminder{ID: "r1", Meta: domainreminders.Metadata{Message: "take pills"}}
	mgr := &fakeManager{listResult: []domainreminders.Reminder{rem}}
	entityRT := &fakeEntityRuntime{}
	base := component.Base{Service: (*ga.Service)(nil)}
	h := hareminders.New(base, tr, mgr, entityRT, "homeapp")
	_ = h.Start(context.Background())

	if len(entityRT.switchKeys) != 1 {
		t.Fatalf("Switch called %d times at startup, want 1", len(entityRT.switchKeys))
	}

	mgr.listResult = nil
	payload, _ := json.Marshal(map[string]string{"id": "r1"})
	tr.dispatch("homeapp/reminders/delete", payload)

	if len(entityRT.removeKeys) != 1 {
		t.Errorf("Remove called %d times after delete, want 1", len(entityRT.removeKeys))
	}
}

func TestHandler_HandleCreate_CallsManagerCreate(t *testing.T) {
	tr := newFakeTransport()
	mgr := &fakeManager{}
	h := newTestHandler(t, tr, mgr)
	_ = h.Start(context.Background())

	payload, _ := json.Marshal(map[string]any{
		"id":           "r1",
		"targets":      []string{"u1"},
		"message":      "take pills",
		"owner":        "u1",
		"source":       "manual",
		"trigger_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
		"requires_ack": true,
		"profile":      "normal",
	})
	tr.dispatch("homeapp/reminders/create", payload)

	if mgr.lastCreate.ID != "r1" {
		t.Errorf("Create called with id=%q, want r1", mgr.lastCreate.ID)
	}
}

func TestHandler_HandleCreate_DeclaresEntity(t *testing.T) {
	tr := newFakeTransport()
	mgr := &fakeManager{}
	entityRT := &fakeEntityRuntime{}
	base := component.Base{Service: (*ga.Service)(nil)}
	h := hareminders.New(base, tr, mgr, entityRT, "homeapp")
	_ = h.Start(context.Background())

	payload, _ := json.Marshal(map[string]any{
		"id":           "r1",
		"targets":      []string{"u1"},
		"message":      "take pills",
		"owner":        "u1",
		"source":       "manual",
		"trigger_at":   time.Now().Add(time.Hour).Format(time.RFC3339),
		"requires_ack": false,
	})
	tr.dispatch("homeapp/reminders/create", payload)

	if len(entityRT.switchKeys) != 1 {
		t.Errorf("Switch called %d times after create, want 1", len(entityRT.switchKeys))
	}
}
