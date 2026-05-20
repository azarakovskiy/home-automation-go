# MQTT Device Grouping in Home Assistant

**Date:** 2026-05-20  
**Status:** Approved

## Problem

MQTT entities published by `home-go` via the HASS MQTT discovery protocol show up ungrouped in the Home Assistant UI. All entities appear as free-floating, with no logical device association.

## Goal

Group entities by logical device:

- Generic (non-device-specific) entities → device `"home-go"`
- Device-specific entities → device `"home-go / <DeviceName>"`

Both the app name and the separator must be configurable via environment variables with sensible defaults.

---

## Design

### 1. Configuration

Two new fields are added to `MQTTConfig` in `internal/config/config.go`:

| Field | Env var | Default |
|---|---|---|
| `AppName` | `MQTT_APP_NAME` | `"home-go"` |
| `DeviceNameSeparator` | `MQTT_DEVICE_NAME_SEPARATOR` | `" / "` |

These are forwarded into `RuntimeConfig` (already passed from `MQTTConfig` to `entities.NewRuntime`).

### 2. MQTT `device` Block

HASS MQTT discovery supports a `device` object in the payload that groups entities under a logical device. `Runtime.baseDiscoveryPayload()` injects this object for every entity.

**Generic entity** (no `ForDevice` call):
```json
"device": {
  "identifiers": ["home-go"],
  "name": "home-go"
}
```

**Device-specific entity** (via `ForDevice("Kitchen Dishwasher")`):
```json
"device": {
  "identifiers": ["home-go_kitchen_dishwasher"],
  "name": "home-go / Kitchen Dishwasher"
}
```

The device identifier is `appPrefix + "_" + slugify(deviceName)` where `slugify` lowercases and replaces spaces with underscores. The generic device identifier is `appPrefix` as-is.

### 3. `EntityDeclarer` Interface and `DeviceRuntime`

A new `EntityDeclarer` interface is introduced in `internal/tech/homeassistant/entities/runtime.go`. Both `*Runtime` and `*DeviceRuntime` satisfy it.

```go
type EntityDeclarer interface {
    Switch(ctx context.Context, spec SwitchSpec) (*SwitchHandle, error)
    NumberSensor(ctx context.Context, spec NumberSensorSpec) (*NumberSensorHandle, error)
    TextSensor(ctx context.Context, spec TextSensorSpec) (*TextSensorHandle, error)
    BinarySensor(ctx context.Context, spec BinarySensorSpec) (*BinarySensorHandle, error)
}
```

`DeviceRuntime` is a new unexported struct:

```go
type DeviceRuntime struct {
    rt         *Runtime
    deviceName string
}

func (r *Runtime) ForDevice(name string) *DeviceRuntime
```

`DeviceRuntime` methods delegate to `rt` but inject the computed device block into the discovery payload.

Internally, `Runtime.declare()` gains an optional `*runtimeDevice` parameter (unexported). `Runtime`'s public methods pass `nil` → app device is used. `DeviceRuntime`'s methods pass the computed `*runtimeDevice`.

```go
type runtimeDevice struct {
    name       string
    identifier string
}
```

### 4. Caller Update Pattern

Each device package calls `runtime.ForDevice(deviceName)` **internally**. Callers in `app.go` continue to pass `*Runtime` unchanged.

Each package defines its own constant for the device name:

```go
// dishwasher/new.go
const deviceName = "Kitchen Dishwasher"

func New(..., runtime *entities.Runtime) (*Dishwasher, error) {
    stateManager, err := NewStateManager(runtime.ForDevice(deviceName), ...)
```

Call sites that accept `*entities.Runtime` and declare entities switch to `entities.EntityDeclarer`:

| File | Change |
|---|---|
| `devices/dishwasher/state_manager.go` | `NewStateManager` accepts `EntityDeclarer` |
| `devices/dishwasher/new.go` | adds `const deviceName = "Kitchen Dishwasher"`; calls `runtime.ForDevice(deviceName)` |
| `devices/health/component.go` | `runtimeAdapter` wraps `EntityDeclarer` instead of `*Runtime`; no `ForDevice` (app device) |
| `devices/reminders/component.go` | `runtimeProjector` keeps wrapping `*Runtime` unchanged — reminder entities are app-level and belong to the generic device; `projector` interface is a superset of `EntityDeclarer` and does not change |

Laptop and vacuum do not declare any MQTT runtime entities (they control HA entities via the HA API directly), so they are unaffected.

`app.go` is untouched.

### 5. Testing

| Test file | What changes |
|---|---|
| `entities/runtime_test.go` | New tests assert the `device` block in published discovery payload — both app device (nil) and named device shapes |
| `devices/health/component_test.go` | `runtimeAdapter` mock satisfies `EntityDeclarer`; structurally unchanged |
| `devices/dishwasher/state_manager_test.go` | Mock/stub satisfies `EntityDeclarer` instead of `*Runtime`; surface unchanged |

No new integration tests required. The discovery payload shape is the right assertion level.

---

## Out of Scope

- Changing existing entity keys, names, or topic structures
- Adding manufacturer/model fields to the device block (can be added later)
- Removing or migrating previously ungrouped entities in existing HASS installations (HASS handles this automatically on next discovery)
