# MQTT Device Grouping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Inject a `device` block into every MQTT discovery payload so Home Assistant groups entities by logical device — generic entities under `"home-go"` and device-specific entities under `"home-go / <DeviceName>"`.

**Architecture:** `RuntimeConfig` gains `AppName` and `DeviceNameSeparator` fields forwarded from `MQTTConfig`. `Runtime.declare()` gains an optional `*runtimeDevice` parameter that controls which device block is injected. `Runtime.ForDevice(name)` returns a `*DeviceRuntime` that wraps the runtime and passes a computed `*runtimeDevice` on every declaration. An `EntityDeclarer` interface lets callers depend on the narrower interface instead of `*Runtime`.

**Tech Stack:** Go 1.23+, `encoding/json`, standard library only

---

### Task 1: Add AppName and DeviceNameSeparator to MQTTConfig

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add fields to MQTTConfig and defaults to Load()**

In `internal/config/config.go`, replace the `MQTTConfig` struct:

```go
type MQTTConfig struct {
	BrokerURL           string
	Username            string
	Password            string
	DiscoveryPrefix     string
	AppPrefix           string
	AppName             string
	DeviceNameSeparator string
}
```

In `Load()`, extend the `MQTT: MQTTConfig{...}` literal with two new fields:

```go
MQTT: MQTTConfig{
    BrokerURL:           os.Getenv("HA_MQTT_BROKER_URL"),
    Username:            os.Getenv("HA_MQTT_USERNAME"),
    Password:            os.Getenv("HA_MQTT_PASSWORD"),
    DiscoveryPrefix:     "homeassistant",
    AppPrefix:           "home-go",
    AppName:             envOrDefault("MQTT_APP_NAME", "home-go"),
    DeviceNameSeparator: envOrDefault("MQTT_DEVICE_NAME_SEPARATOR", " / "),
},
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/config/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -m "feat(config): add AppName and DeviceNameSeparator to MQTTConfig"
```

---

### Task 2: Add device-block support to Runtime (app-level device)

**Files:**
- Modify: `internal/tech/homeassistant/entities/runtime.go`
- Modify: `internal/tech/homeassistant/entities/runtime_test.go`

- [ ] **Step 1: Write failing test for app device block**

Append to `internal/tech/homeassistant/entities/runtime_test.go`:

```go
func TestRuntimeDiscoveryPayloadContainsAppDevice(t *testing.T) {
	transport := newFakeRuntimeTransport()
	rt := newTestRuntime(t, transport, "")

	_, err := rt.Switch(context.Background(), SwitchSpec{
		CommonSpec: CommonSpec{Key: "feature_x", Name: "Feature X"},
	})
	if err != nil {
		t.Fatalf("Switch() error = %v", err)
	}

	pub := transport.lastPublish("homeassistant/switch/feature_x/config")
	var payload map[string]any
	if err := json.Unmarshal(pub.payload, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	device, ok := payload["device"].(map[string]any)
	if !ok {
		t.Fatalf("device block missing or wrong type, got %T", payload["device"])
	}
	ids, ok := device["identifiers"].([]any)
	if !ok || len(ids) != 1 || ids[0] != "home-go" {
		t.Fatalf("device.identifiers = %v, want [home-go]", device["identifiers"])
	}
	if device["name"] != "home-go" {
		t.Fatalf("device.name = %v, want home-go", device["name"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tech/homeassistant/entities/... -run TestRuntimeDiscoveryPayloadContainsAppDevice -v`
Expected: FAIL — `device` key is missing from the payload

- [ ] **Step 3: Add AppName and DeviceNameSeparator to RuntimeConfig**

In `internal/tech/homeassistant/entities/runtime.go`, replace `RuntimeConfig`:

```go
type RuntimeConfig struct {
	BrokerURL           string
	Username            string
	Password            string
	DiscoveryPrefix     string
	AppPrefix           string
	ClientID            string
	RegistryPath        string
	AppName             string
	DeviceNameSeparator string
}
```

- [ ] **Step 4: Add appName and deviceNameSeparator fields to Runtime struct**

Replace the `Runtime` struct:

```go
type Runtime struct {
	mqtt runtimeTransport

	discoveryPrefix     string
	appPrefix           string
	appName             string
	deviceNameSeparator string
	availabilityTopic   string
	haStatusTopic       string
	commandTopic        string

	registry *runtimeRegistry

	mu             sync.RWMutex
	entities       map[string]*runtimeEntity
	switchHandlers map[string]func(context.Context, bool) error
}
```

- [ ] **Step 5: Default and store the new fields in NewRuntime()**

In `NewRuntime()`, add these two lines after the existing validation block (after the `AppPrefix` check) and before `registry, err := ...`:

```go
if strings.TrimSpace(cfg.AppName) == "" {
    cfg.AppName = cfg.AppPrefix
}
if cfg.DeviceNameSeparator == "" {
    cfg.DeviceNameSeparator = " / "
}
```

In the `rt := &Runtime{...}` literal, add the two new fields after `appPrefix`:

```go
appPrefix:           cfg.AppPrefix,
appName:             cfg.AppName,
deviceNameSeparator: cfg.DeviceNameSeparator,
```

- [ ] **Step 6: Add runtimeDevice type and slugify helper**

After the `runtimeEntity` struct definition, add:

```go
type runtimeDevice struct {
	name       string
	identifier string
}

func slugify(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "_")
}
```

- [ ] **Step 7: Change declare() to accept an optional device**

Change the `declare()` signature:

```go
func (r *Runtime) declare(ctx context.Context, kind runtimeEntityKind, spec CommonSpec, payload map[string]any, device *runtimeDevice) (*runtimeEntity, error) {
```

Inside `declare()`, update the `mergeDiscoveryPayload` call:

```go
discoveryPayload, err := json.Marshal(mergeDiscoveryPayload(payload, r.baseDiscoveryPayload(spec, entity, device)))
```

- [ ] **Step 8: Inject the device block in baseDiscoveryPayload()**

Change the `baseDiscoveryPayload` signature:

```go
func (r *Runtime) baseDiscoveryPayload(spec CommonSpec, entity *runtimeEntity, device *runtimeDevice) map[string]any {
```

At the end of `baseDiscoveryPayload()`, before the `return payload` line, add:

```go
deviceID := r.appName
deviceDisplayName := r.appName
if device != nil {
    deviceID = device.identifier
    deviceDisplayName = device.name
}
payload["device"] = map[string]any{
    "identifiers": []string{deviceID},
    "name":        deviceDisplayName,
}
```

- [ ] **Step 9: Update the four public Runtime methods to pass nil for device**

In `Runtime.Switch()`, change the `declare` call:

```go
entity, err := r.declare(ctx, runtimeKindSwitch, spec.CommonSpec, switchDiscoveryPayload(), nil)
```

In `Runtime.NumberSensor()`:

```go
entity, err := r.declare(ctx, runtimeKindSensor, spec.CommonSpec, numberSensorDiscoveryPayload(spec), nil)
```

In `Runtime.TextSensor()`:

```go
entity, err := r.declare(ctx, runtimeKindSensor, spec.CommonSpec, textSensorDiscoveryPayload(), nil)
```

In `Runtime.BinarySensor()`:

```go
entity, err := r.declare(ctx, runtimeKindBinarySensor, spec.CommonSpec, binarySensorDiscoveryPayload(), nil)
```

- [ ] **Step 10: Update newTestRuntime to populate appName and deviceNameSeparator**

In `runtime_test.go`, inside `newTestRuntime`, add the two new fields to the `Runtime` struct literal:

```go
rt := &Runtime{
    mqtt:                transport,
    discoveryPrefix:     "homeassistant",
    appPrefix:           "home-go",
    appName:             "home-go",
    deviceNameSeparator: " / ",
    availabilityTopic:   "home-go/status",
    haStatusTopic:       "homeassistant/status",
    commandTopic:        "home-go/entities/+/set",
    registry:            registry,
    entities:            make(map[string]*runtimeEntity),
    switchHandlers:      make(map[string]func(context.Context, bool) error),
}
```

- [ ] **Step 11: Run all entity tests**

Run: `go test ./internal/tech/homeassistant/entities/... -v`
Expected: all PASS, including `TestRuntimeDiscoveryPayloadContainsAppDevice`

- [ ] **Step 12: Commit**

```bash
git add internal/tech/homeassistant/entities/runtime.go internal/tech/homeassistant/entities/runtime_test.go
git commit -m "feat(entities): inject device block into MQTT discovery payloads"
```

---

### Task 3: Add EntityDeclarer interface, DeviceRuntime, and ForDevice

**Files:**
- Modify: `internal/tech/homeassistant/entities/runtime.go`
- Modify: `internal/tech/homeassistant/entities/runtime_test.go`

- [ ] **Step 1: Write failing test for ForDevice device block**

Append to `internal/tech/homeassistant/entities/runtime_test.go`:

```go
func TestRuntimeForDeviceDiscoveryPayloadContainsNamedDevice(t *testing.T) {
	transport := newFakeRuntimeTransport()
	rt := newTestRuntime(t, transport, "")
	dr := rt.ForDevice("Kitchen Dishwasher")

	_, err := dr.Switch(context.Background(), SwitchSpec{
		CommonSpec: CommonSpec{
			Key:  "kitchen_dishwasher_is_scheduled",
			Name: "Kitchen Dishwasher: Is Scheduled",
		},
	})
	if err != nil {
		t.Fatalf("Switch() error = %v", err)
	}

	pub := transport.lastPublish("homeassistant/switch/kitchen_dishwasher_is_scheduled/config")
	var payload map[string]any
	if err := json.Unmarshal(pub.payload, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	device, ok := payload["device"].(map[string]any)
	if !ok {
		t.Fatalf("device block missing or wrong type")
	}
	ids, ok := device["identifiers"].([]any)
	if !ok || len(ids) != 1 || ids[0] != "home-go_kitchen_dishwasher" {
		t.Fatalf("device.identifiers = %v, want [home-go_kitchen_dishwasher]", device["identifiers"])
	}
	if device["name"] != "home-go / Kitchen Dishwasher" {
		t.Fatalf("device.name = %v, want %q", device["name"], "home-go / Kitchen Dishwasher")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tech/homeassistant/entities/... -run TestRuntimeForDeviceDiscoveryPayloadContainsNamedDevice -v`
Expected: compile error — `ForDevice` undefined

- [ ] **Step 3: Add EntityDeclarer interface**

In `runtime.go`, after the `BinarySensorHandle` type definition, add:

```go
// EntityDeclarer is the entity-declaration subset of *Runtime.
// Both *Runtime and *DeviceRuntime satisfy this interface.
type EntityDeclarer interface {
	Switch(ctx context.Context, spec SwitchSpec) (*SwitchHandle, error)
	NumberSensor(ctx context.Context, spec NumberSensorSpec) (*NumberSensorHandle, error)
	TextSensor(ctx context.Context, spec TextSensorSpec) (*TextSensorHandle, error)
	BinarySensor(ctx context.Context, spec BinarySensorSpec) (*BinarySensorHandle, error)
}
```

- [ ] **Step 4: Add DeviceRuntime, ForDevice, and its four methods**

After the `EntityDeclarer` interface, add:

```go
// DeviceRuntime wraps Runtime and injects a named device block into every declaration.
type DeviceRuntime struct {
	rt         *Runtime
	deviceName string
}

// ForDevice returns a DeviceRuntime that groups all declared entities under
// the named device (e.g., "Kitchen Dishwasher" → "home-go / Kitchen Dishwasher").
func (r *Runtime) ForDevice(name string) *DeviceRuntime {
	return &DeviceRuntime{rt: r, deviceName: name}
}

func (dr *DeviceRuntime) device() *runtimeDevice {
	return &runtimeDevice{
		identifier: dr.rt.appName + "_" + slugify(dr.deviceName),
		name:       dr.rt.appName + dr.rt.deviceNameSeparator + dr.deviceName,
	}
}

func (dr *DeviceRuntime) Switch(ctx context.Context, spec SwitchSpec) (*SwitchHandle, error) {
	if err := validateCommonSpec(spec.CommonSpec, "switch"); err != nil {
		return nil, err
	}
	entity, err := dr.rt.declare(ctx, runtimeKindSwitch, spec.CommonSpec, switchDiscoveryPayload(), dr.device())
	if err != nil {
		return nil, err
	}
	return &SwitchHandle{runtime: dr.rt, key: entity.key}, nil
}

func (dr *DeviceRuntime) NumberSensor(ctx context.Context, spec NumberSensorSpec) (*NumberSensorHandle, error) {
	if err := validateCommonSpec(spec.CommonSpec, "number sensor"); err != nil {
		return nil, err
	}
	entity, err := dr.rt.declare(ctx, runtimeKindSensor, spec.CommonSpec, numberSensorDiscoveryPayload(spec), dr.device())
	if err != nil {
		return nil, err
	}
	return &NumberSensorHandle{runtime: dr.rt, key: entity.key}, nil
}

func (dr *DeviceRuntime) TextSensor(ctx context.Context, spec TextSensorSpec) (*TextSensorHandle, error) {
	if err := validateCommonSpec(spec.CommonSpec, "text sensor"); err != nil {
		return nil, err
	}
	entity, err := dr.rt.declare(ctx, runtimeKindSensor, spec.CommonSpec, textSensorDiscoveryPayload(), dr.device())
	if err != nil {
		return nil, err
	}
	return &TextSensorHandle{runtime: dr.rt, key: entity.key}, nil
}

func (dr *DeviceRuntime) BinarySensor(ctx context.Context, spec BinarySensorSpec) (*BinarySensorHandle, error) {
	if err := validateCommonSpec(spec.CommonSpec, "binary sensor"); err != nil {
		return nil, err
	}
	entity, err := dr.rt.declare(ctx, runtimeKindBinarySensor, spec.CommonSpec, binarySensorDiscoveryPayload(), dr.device())
	if err != nil {
		return nil, err
	}
	return &BinarySensorHandle{runtime: dr.rt, key: entity.key}, nil
}
```

- [ ] **Step 5: Run all entity tests**

Run: `go test ./internal/tech/homeassistant/entities/... -v`
Expected: all PASS, including `TestRuntimeForDeviceDiscoveryPayloadContainsNamedDevice`

- [ ] **Step 6: Commit**

```bash
git add internal/tech/homeassistant/entities/runtime.go internal/tech/homeassistant/entities/runtime_test.go
git commit -m "feat(entities): add EntityDeclarer interface, DeviceRuntime, and ForDevice"
```

---

### Task 4: Dishwasher — adopt EntityDeclarer and ForDevice

**Files:**
- Modify: `internal/tech/homeassistant/devices/dishwasher/state_manager.go`
- Modify: `internal/tech/homeassistant/devices/dishwasher/new.go`

- [ ] **Step 1: Change NewStateManager to accept EntityDeclarer**

In `internal/tech/homeassistant/devices/dishwasher/state_manager.go`, change the function signature:

```go
func NewStateManager(runtime entities.EntityDeclarer, state ga.State, controller *Controller) (*StateManager, error) {
```

The nil check and all `runtime.Switch(...)`, `runtime.TextSensor(...)`, and `runtime.NumberSensor(...)` calls in the body are unchanged — all three methods are part of `EntityDeclarer`.

- [ ] **Step 2: Add deviceName constant and call ForDevice in new.go**

In `internal/tech/homeassistant/devices/dishwasher/new.go`, add the constant and update `New()`:

```go
const deviceName = "Kitchen Dishwasher"

func New(base component.Base, state ga.State, priceService *domainpricing.Service, runtime *entities.Runtime) (*domaindishwasher.Dishwasher, error) {
	controller := NewController(base.Service)
	stateManager, err := NewStateManager(runtime.ForDevice(deviceName), state, controller)
	if err != nil {
		return nil, fmt.Errorf("create state manager: %w", err)
	}
	// rest of function unchanged
```

- [ ] **Step 3: Run dishwasher tests**

Run: `go test ./internal/tech/homeassistant/devices/dishwasher/... -v`
Expected: all PASS (`TestNewStateManagerRequiresRuntime` passes `nil` which is a nil `EntityDeclarer` — the nil check still works)

- [ ] **Step 4: Commit**

```bash
git add internal/tech/homeassistant/devices/dishwasher/state_manager.go internal/tech/homeassistant/devices/dishwasher/new.go
git commit -m "feat(dishwasher): group entities under Kitchen Dishwasher device"
```

---

### Task 5: Health component — widen runtimeAdapter to EntityDeclarer

**Files:**
- Modify: `internal/tech/homeassistant/devices/health/component.go`

- [ ] **Step 1: Change runtimeAdapter and New() to use EntityDeclarer**

In `internal/tech/homeassistant/devices/health/component.go`, change `runtimeAdapter`:

```go
type runtimeAdapter struct {
	rt entities.EntityDeclarer
}
```

Change `New()` parameter type:

```go
func New(ctx context.Context, base component.Base, runtime entities.EntityDeclarer, startTime time.Time) (*Component, error) {
```

The rest of the function body is unchanged — `adapter.textSensor(...)` calls `r.rt.TextSensor(...)`, which is still present on `EntityDeclarer`.

- [ ] **Step 2: Run health tests**

Run: `go test ./internal/tech/homeassistant/devices/health/... -v`
Expected: all PASS (tests construct `Component` directly, bypassing `New()`)

- [ ] **Step 3: Commit**

```bash
git add internal/tech/homeassistant/devices/health/component.go
git commit -m "feat(health): widen runtimeAdapter and New() to EntityDeclarer"
```

---

### Task 6: Forward new config fields in app.go and final verification

**Files:**
- Modify: `internal/app.go`

- [ ] **Step 1: Add AppName and DeviceNameSeparator to the RuntimeConfig literal**

In `internal/app.go`, extend the `entities.RuntimeConfig{...}` literal:

```go
runtimeEntities, err := entities.NewRuntime(entities.RuntimeConfig{
    BrokerURL:           cfg.MQTT.BrokerURL,
    Username:            cfg.MQTT.Username,
    Password:            cfg.MQTT.Password,
    DiscoveryPrefix:     cfg.MQTT.DiscoveryPrefix,
    AppPrefix:           cfg.MQTT.AppPrefix,
    AppName:             cfg.MQTT.AppName,
    DeviceNameSeparator: cfg.MQTT.DeviceNameSeparator,
})
```

- [ ] **Step 2: Full build**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 3: Full test suite**

Run: `go test ./...`
Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add internal/app.go
git commit -m "feat(app): forward AppName and DeviceNameSeparator to RuntimeConfig"
```
