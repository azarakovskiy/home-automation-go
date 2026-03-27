package entities

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRuntimeDeclareDoesNotPublishState(t *testing.T) {
	transport := newFakeRuntimeTransport()
	rt := newTestRuntime(t, transport, "")

	_, err := rt.NumberSensor(context.Background(), NumberSensorSpec{
		CommonSpec: CommonSpec{
			Key:          "dishwasher_savings",
			Name:         "Dishwasher Savings",
			EntityIDHint: "sensor.kitchen_dishwasher_savings",
		},
		UnitOfMeasurement: "EUR",
	})
	if err != nil {
		t.Fatalf("NumberSensor() error = %v", err)
	}

	if got := transport.publishCount("homeassistant/sensor/dishwasher_savings/config"); got != 1 {
		t.Fatalf("discovery publish count = %d, want 1", got)
	}
	if got := transport.publishCount("home-go/entities/dishwasher_savings/state"); got != 0 {
		t.Fatalf("state publish count = %d, want 0", got)
	}
}

func TestRuntimeSetPublishesRetainedState(t *testing.T) {
	transport := newFakeRuntimeTransport()
	rt := newTestRuntime(t, transport, "")

	sensor, err := rt.NumberSensor(context.Background(), NumberSensorSpec{
		CommonSpec: CommonSpec{
			Key:  "dishwasher_savings",
			Name: "Dishwasher Savings",
		},
		UnitOfMeasurement: "EUR",
	})
	if err != nil {
		t.Fatalf("NumberSensor() error = %v", err)
	}
	if err := sensor.Set(context.Background(), 1.42); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	pub := transport.lastPublish("home-go/entities/dishwasher_savings/state")
	if string(pub.payload) != "1.42" {
		t.Fatalf("state payload = %q, want %q", string(pub.payload), "1.42")
	}
	if !pub.retained {
		t.Fatalf("state retained = false, want true")
	}
}

func TestRuntimeRemoveClearsDiscoveryAndState(t *testing.T) {
	transport := newFakeRuntimeTransport()
	registryPath := filepath.Join(t.TempDir(), "registry.json")
	rt := newTestRuntime(t, transport, registryPath)

	sensor, err := rt.NumberSensor(context.Background(), NumberSensorSpec{
		CommonSpec: CommonSpec{
			Key:  "dishwasher_savings",
			Name: "Dishwasher Savings",
		},
	})
	if err != nil {
		t.Fatalf("NumberSensor() error = %v", err)
	}
	if err := sensor.Set(context.Background(), 1.42); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := sensor.Remove(context.Background()); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if payload := transport.lastPublish("homeassistant/sensor/dishwasher_savings/config").payload; payload != nil {
		t.Fatalf("discovery payload = %v, want nil", payload)
	}
	if payload := transport.lastPublish("home-go/entities/dishwasher_savings/state").payload; payload != nil {
		t.Fatalf("state payload = %v, want nil", payload)
	}
}

func TestRuntimeSwitchCommandDispatch(t *testing.T) {
	transport := newFakeRuntimeTransport()
	rt := newTestRuntime(t, transport, "")

	sw, err := rt.Switch(context.Background(), SwitchSpec{
		CommonSpec: CommonSpec{
			Key:  "feature_dishwasher_auto",
			Name: "Dishwasher Auto Scheduling",
		},
	})
	if err != nil {
		t.Fatalf("Switch() error = %v", err)
	}

	var (
		mu   sync.Mutex
		got  bool
		done = make(chan struct{}, 1)
	)
	if err := sw.OnCommand(func(_ context.Context, on bool) error {
		mu.Lock()
		got = on
		mu.Unlock()
		done <- struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("OnCommand() error = %v", err)
	}

	transport.emit("home-go/entities/feature_dishwasher_auto/set", []byte("ON"))

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("command handler was not called")
	}

	mu.Lock()
	defer mu.Unlock()
	if !got {
		t.Fatalf("handler got = false, want true")
	}
}

func TestRuntimeReconcileRemovesStaleEntries(t *testing.T) {
	transport := newFakeRuntimeTransport()
	registryPath := filepath.Join(t.TempDir(), "registry.json")
	rt := newTestRuntime(t, transport, registryPath)

	_, err := rt.NumberSensor(context.Background(), NumberSensorSpec{
		CommonSpec: CommonSpec{Key: "keep", Name: "Keep"},
	})
	if err != nil {
		t.Fatalf("NumberSensor() keep error = %v", err)
	}
	_, err = rt.NumberSensor(context.Background(), NumberSensorSpec{
		CommonSpec: CommonSpec{Key: "remove", Name: "Remove"},
	})
	if err != nil {
		t.Fatalf("NumberSensor() remove error = %v", err)
	}

	if err := rt.Reconcile(context.Background(), []string{"keep"}); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if payload := transport.lastPublish("homeassistant/sensor/remove/config").payload; payload != nil {
		t.Fatalf("removed discovery payload = %v, want nil", payload)
	}
	if _, exists := rt.registry.Kind("remove"); exists {
		t.Fatalf("registry still contains removed key")
	}
}

func TestRuntimeDeclareWithDifferentKindsFails(t *testing.T) {
	transport := newFakeRuntimeTransport()
	rt := newTestRuntime(t, transport, "")

	_, err := rt.Switch(context.Background(), SwitchSpec{
		CommonSpec: CommonSpec{Key: "feature_x", Name: "Feature X"},
	})
	if err != nil {
		t.Fatalf("Switch() error = %v", err)
	}

	_, err = rt.NumberSensor(context.Background(), NumberSensorSpec{
		CommonSpec: CommonSpec{Key: "feature_x", Name: "Feature X"},
	})
	if err == nil {
		t.Fatal("NumberSensor() error = nil, want error")
	}
}

func TestRuntimeReconnectRepublishesKnownState(t *testing.T) {
	transport := newFakeRuntimeTransport()
	rt := newTestRuntime(t, transport, "")

	sensor, err := rt.NumberSensor(context.Background(), NumberSensorSpec{
		CommonSpec: CommonSpec{Key: "dishwasher_savings", Name: "Dishwasher Savings"},
	})
	if err != nil {
		t.Fatalf("NumberSensor() error = %v", err)
	}
	if err := sensor.Set(context.Background(), 2.5); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if err := rt.handleReconnect(context.Background()); err != nil {
		t.Fatalf("handleReconnect() error = %v", err)
	}

	if got := transport.publishCount("homeassistant/sensor/dishwasher_savings/config"); got < 2 {
		t.Fatalf("discovery publish count = %d, want at least 2", got)
	}
	if got := transport.publishCount("home-go/entities/dishwasher_savings/state"); got < 2 {
		t.Fatalf("state publish count = %d, want at least 2", got)
	}
}

func TestRuntimeDiscoveryPayloadContainsMetadata(t *testing.T) {
	transport := newFakeRuntimeTransport()
	rt := newTestRuntime(t, transport, "")

	_, err := rt.Switch(context.Background(), SwitchSpec{
		CommonSpec: CommonSpec{
			Key:          "feature_x",
			Name:         "Feature X",
			EntityIDHint: "switch.feature_x",
			Icon:         "mdi:tune",
		},
	})
	if err != nil {
		t.Fatalf("Switch() error = %v", err)
	}

	pub := transport.lastPublish("homeassistant/switch/feature_x/config")
	var payload map[string]any
	if err := json.Unmarshal(pub.payload, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if payload["default_entity_id"] != "switch.feature_x" {
		t.Fatalf("default_entity_id = %v, want switch.feature_x", payload["default_entity_id"])
	}
	if payload["command_topic"] != "home-go/entities/feature_x/set" {
		t.Fatalf("command_topic = %v, want home-go/entities/feature_x/set", payload["command_topic"])
	}
}

func TestParseRuntimeCommandTopic(t *testing.T) {
	key, err := parseRuntimeCommandTopic("home-go", "home-go/entities/feature_x/set")
	if err != nil {
		t.Fatalf("parseRuntimeCommandTopic() error = %v", err)
	}
	if key != "feature_x" {
		t.Fatalf("key = %q, want feature_x", key)
	}
}

func newTestRuntime(t *testing.T, transport runtimeTransport, registryPath string) *Runtime {
	t.Helper()

	registry, err := newRuntimeRegistry(registryPath)
	if err != nil {
		t.Fatalf("newRuntimeRegistry() error = %v", err)
	}

	rt := &Runtime{
		mqtt:              transport,
		discoveryPrefix:   "homeassistant",
		appPrefix:         "home-go",
		availabilityTopic: "home-go/status",
		haStatusTopic:     "homeassistant/status",
		commandTopic:      "home-go/entities/+/set",
		registry:          registry,
		entities:          make(map[string]*runtimeEntity),
		switchHandlers:    make(map[string]func(context.Context, bool) error),
	}

	if err := transport.Subscribe(context.Background(), rt.commandTopic, rt.handleCommand); err != nil {
		t.Fatalf("Subscribe(commandTopic) error = %v", err)
	}
	if err := transport.Subscribe(context.Background(), rt.haStatusTopic, rt.handleHomeAssistantStatus); err != nil {
		t.Fatalf("Subscribe(haStatusTopic) error = %v", err)
	}

	return rt
}

type fakeRuntimeTransport struct {
	mu            sync.Mutex
	onConnect     func(context.Context) error
	subscriptions map[string]runtimeMessageHandler
	publishes     map[string][]fakePublish
}

type fakePublish struct {
	retained bool
	payload  []byte
}

func newFakeRuntimeTransport() *fakeRuntimeTransport {
	return &fakeRuntimeTransport{
		subscriptions: make(map[string]runtimeMessageHandler),
		publishes:     make(map[string][]fakePublish),
	}
}

func (f *fakeRuntimeTransport) SetOnConnect(fn func(context.Context) error) {
	f.onConnect = fn
}

func (f *fakeRuntimeTransport) Connect(context.Context) error {
	if f.onConnect != nil {
		return f.onConnect(context.Background())
	}
	return nil
}

func (f *fakeRuntimeTransport) Publish(_ context.Context, topic string, retained bool, payload []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	cloned := append([]byte(nil), payload...)
	f.publishes[topic] = append(f.publishes[topic], fakePublish{
		retained: retained,
		payload:  cloned,
	})
	return nil
}

func (f *fakeRuntimeTransport) Subscribe(_ context.Context, topic string, handler runtimeMessageHandler) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.subscriptions[topic] = handler
	return nil
}

func (f *fakeRuntimeTransport) Close() error {
	return nil
}

func (f *fakeRuntimeTransport) emit(topic string, payload []byte) {
	f.mu.Lock()
	handlers := make([]runtimeMessageHandler, 0, len(f.subscriptions))
	for pattern, handler := range f.subscriptions {
		if topicMatches(pattern, topic) {
			handlers = append(handlers, handler)
		}
	}
	f.mu.Unlock()

	for _, handler := range handlers {
		handler(context.Background(), topic, payload)
	}
}

func (f *fakeRuntimeTransport) publishCount(topic string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.publishes[topic])
}

func (f *fakeRuntimeTransport) lastPublish(topic string) fakePublish {
	f.mu.Lock()
	defer f.mu.Unlock()

	publishes := f.publishes[topic]
	if len(publishes) == 0 {
		return fakePublish{}
	}
	return publishes[len(publishes)-1]
}

func topicMatches(pattern, topic string) bool {
	if pattern == topic {
		return true
	}
	patternParts := strings.Split(pattern, "/")
	topicParts := strings.Split(topic, "/")
	if len(patternParts) != len(topicParts) {
		return false
	}
	for i := range patternParts {
		if patternParts[i] == "+" {
			continue
		}
		if patternParts[i] != topicParts[i] {
			return false
		}
	}
	return true
}

func TestParseRuntimeBoolPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    bool
		wantErr bool
	}{
		{name: "on", payload: []byte("ON"), want: true},
		{name: "off", payload: []byte("OFF"), want: false},
		{name: "invalid", payload: []byte("maybe"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRuntimeBoolPayload(tt.payload)
			if tt.wantErr {
				if err == nil {
					t.Fatal("parseRuntimeBoolPayload() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRuntimeBoolPayload() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("parseRuntimeBoolPayload() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestRuntimeReconcileRequiresRegistry(t *testing.T) {
	rt := newTestRuntime(t, newFakeRuntimeTransport(), "")
	err := rt.Reconcile(context.Background(), []string{"keep"})
	if err == nil {
		t.Fatal("Reconcile() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "reconcile requires registry path") {
		t.Fatalf("Reconcile() error = %v, want registry path error", err)
	}
}
