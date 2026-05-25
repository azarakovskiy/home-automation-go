# Reminders HA Entities Design

## Goal

Expose every active reminder as a native Home Assistant switch entity so users can view and act on reminders directly from a Lovelace dashboard without writing HA automations.

## Architecture

Four bounded changes, no new packages:

### 1. `internal/tech/homeassistant/entities` — two new capabilities

**`SwitchHandle.Set(ctx context.Context, on bool) error`**
Publishes the current ON/OFF state to the entity's state topic. Mirrors `TextSensorHandle.Set`. Required for updating switch state when a reminder fires or is rescheduled.

**`DeviceRuntime.Remove(ctx context.Context, key string) error`**
Publishes an empty payload to the entity's discovery topic (HA interprets this as entity removal), then removes the entity from the internal map. Required for cleaning up entities when reminders are deleted or completed.

### 2. `internal/domain/reminders` — List operation

**`Repository` interface** gains `List(ctx context.Context) ([]Reminder, error)`.

**`Manager`** gains `List(ctx context.Context) ([]Reminder, error)` — thin delegation to the repo. Used by the handler for startup sync and post-tick reconciliation.

**`Trigger` fix for recurring reminders**: `Trigger` must reset `Acks` to nil when `Schedule.Kind == ScheduleKindRecurring`. Without this, `IsComplete()` returns true on the second firing without a new ack, breaking the pending-ack detection logic.

### 3. `internal/tech/postgres` — ListAll query

New sqlc query `ListAll` returns all rows from `reminders`, `reminder_targets`, and `reminder_acks`. Repo implements `Repository.List` using it.

### 4. `internal/tech/homeassistant/devices/reminders` — entity sync

The `Handler` gains:
- A `*entities.DeviceRuntime` (from `runtimeEntities.ForDevice("Reminders")`) injected via `New()`
- An in-memory `map[ReminderID]switchEntry` guarded by a `sync.RWMutex`; each entry holds the `*SwitchHandle` and a snapshot of the reminder's state
- `syncEntities(ctx)` — reconciles the map against `manager.List()`:
  - Reminder in DB, not in map → declare switch, register command callback, store entry
  - Reminder in map, not in DB → `DeviceRuntime.Remove`, delete from map
  - Reminder in both → call `SwitchHandle.Set` with updated state
- `declareEntity(ctx, rem)` — declares one switch via `DeviceRuntime.Switch`, registers its command callback, stores in map
- `entityState(rem) bool` — returns `false` (OFF) when `rem.Policy.RequiresAck && rem.State.FireCount > 0`; `true` (ON) otherwise

`app.go` passes `runtimeEntities` into `hareminders.New()`.

## Entity model

- **Device name**: `"Reminders"` → appears in HA as `home-go / Reminders`
- **One switch per reminder**
  - Key: `reminder_` + reminder ID (slugified)
  - Name: reminder's `Message` field (truncated to 64 chars)
  - Icon: `mdi:bell` (fixed at declaration)
  - Attribute `reminder_id`: the raw reminder ID — used by the HA dashboard hold action
- **State**:
  - `ON` — reminder exists and is not awaiting acknowledgment (scheduled, not yet fired; or recurring, rescheduled after last ack)
  - `OFF` — reminder has fired and is waiting for acknowledgment (`RequiresAck && FireCount > 0`)

## Command handling

**Tap (switch command topic, value `"ON"`):**
The chip was OFF (pending ack). Backend calls `manager.Ack(ctx, id, reminder.Owner)`.
- Once reminder: acked → `IsComplete()` true → manager removes it → next sync removes entity
- Recurring reminder: acked → stays in DB with new `NextRunAt` → next sync sets entity back to ON

**Tap (switch command topic, value `"OFF"`):**
The chip was ON (scheduled). Backend takes no action — reminder will fire at its scheduled time.

**Hold (separate `mqtt.publish` to `{prefix}/reminders/delete`):**
The HA dashboard `hold_action` publishes `{"id": "<reminder_id>"}` to the existing delete topic. Backend deletes the reminder unconditionally. This is indistinguishable from any other delete message — the backend has no knowledge of the gesture.

The `reminder_id` entity attribute makes this possible without hardcoding IDs in HA.

## Lifecycle

```
Start(ctx)
  manager.List → for each reminder: declareEntity

handleCreate → manager.Create → declareEntity

handleAck    → manager.Ack    → syncEntities
handleDelete → manager.Delete → syncEntities

tick         → manager.Tick   → syncEntities
```

`syncEntities` after every mutating operation ensures consistency even if a direct path errors mid-way.

## Error handling

- Entity declaration errors at startup: log and continue — a missing chip is preferable to a failed startup
- `SwitchHandle.Set` errors: log
- `syncEntities` errors: log; stale state resolves on the next tick

## Testing

- **Unit**: extend `handler_test.go` — mock `reminderManager` with `List`; fake `DeviceRuntime` (new interface `entityRuntime`) tracking declare/remove/set calls; assert sync behaviour on create, ack, delete, tick
- **Integration**: existing `reminders_repo_test.go` extended with `List` test

## HA dashboard usage

### What appears

All active reminders appear as switch entities under the device `home-go / Reminders` in HA. No manual entity configuration is needed — they appear and disappear automatically as reminders are created, acknowledged, or deleted.

### Dashboard setup

Install [auto-entities](https://github.com/thomasloven/lovelace-auto-entities) and [lovelace-ui-minimalist](https://ui-lovelace-minimalist.github.io/) if not already present.

Add a card to your dashboard view:

```yaml
type: custom:auto-entities
filter:
  include:
    - device: home-go / Reminders
      domain: switch
card:
  type: custom:mushroom-chips-card
card_param: chips
item_config:
  type: template
  icon: mdi:bell
  tap_action:
    action: toggle
  hold_action:
    action: call-service
    service: mqtt.publish
    data:
      topic: homeautomation/reminders/delete
      payload: >-
        {"id": "{{ state_attr(config.entity, 'reminder_id') }}"}
```

Replace `homeautomation` with your configured MQTT app prefix.

### Chip behaviour

| Chip appearance | Meaning | Tap | Hold |
|---|---|---|---|
| Lit (ON) | Scheduled — will fire at its trigger time | No-op | Delete permanently |
| Dimmed (OFF) | Fired — waiting for acknowledgment | Acknowledge | Delete permanently |

### Creating reminders from HA

Use the existing MQTT API (see `docs/reminders-mqtt-api.md`):

```yaml
action: mqtt.publish
data:
  topic: homeautomation/reminders/create
  payload: >-
    {
      "id": "daily-meds",
      "targets": ["alexey"],
      "message": "Take your vitamins",
      "trigger_at": "2026-05-26T08:00:00Z",
      "recur_every": "24h",
      "requires_ack": true,
      "profile": "normal"
    }
```

The chip appears on the dashboard within one minute (next tick).
