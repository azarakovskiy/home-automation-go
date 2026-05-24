package reminders_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	domainreminders "home-go/internal/domain/reminders"
	hareminders "home-go/internal/tech/homeassistant/devices/reminders"
	"home-go/internal/tech/homeassistant/component"

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
}

func (f *fakeManager) Create(_ context.Context, cmd domainreminders.CreateCommand) (domainreminders.Reminder, error) {
	f.lastCreate = cmd
	return domainreminders.Reminder{}, nil
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

func newTestHandler(t *testing.T, transport *fakeTransport, mgr *fakeManager) *hareminders.Handler {
	t.Helper()
	base := component.Base{Service: (*ga.Service)(nil)}
	return hareminders.New(base, transport, mgr, "homeapp")
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
