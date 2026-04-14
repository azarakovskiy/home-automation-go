# Reminders — Home Assistant Integration Guide

This document explains how to create, acknowledge, and delete reminders from Home Assistant automations and scripts. All communication uses HA custom events.

## Event Reference

| Event | Purpose |
|---|---|
| `custom_reminder_create` | Create a new reminder |
| `custom_reminder_ack` | Acknowledge a reminder for one user |
| `custom_reminder_delete` | Delete a reminder |

---

## Creating a Reminder

Fire the `custom_reminder_create` event with the following payload.

### Required fields

| Field | Type | Description |
|---|---|---|
| `id` | string | Stable, caller-assigned identifier. Must be unique. Used for ack and delete. |
| `targets` | list of strings | User IDs that receive the reminder. At least one required. |
| `message` | string | Human-readable reminder text. |
| `trigger_at` | string (RFC3339) | When the reminder fires for the first time. |
| `requires_ack` | boolean | Whether users must explicitly acknowledge the reminder. |

### Optional fields

| Field | Type | Default | Description |
|---|---|---|---|
| `owner` | string | `""` | Who created the reminder (informational). |
| `source` | string | `""` | Which automation or subsystem created it (informational). |
| `recur_every` | string (Go duration) | `""` (once) | Recurrence interval, e.g. `"1h"`, `"30m"`, `"24h"`. If set, the reminder repeats. |
| `valid_until` | string (RFC3339) | `""` (never expires) | Stop firing after this time. |
| `completion_policy` | string | `"all_targets_ack"` | `"all_targets_ack"` or `"any_target_ack"`. |
| `profile` | string | `"normal"` | `"quiet"`, `"normal"`, or `"annoying"`. Controls repeat cadence for ack-required reminders. |

### Profiles and repeat cadence

When `requires_ack: true`, the reminder repeats until acknowledged. Profile controls timing:

| Profile | Initial delay | Repeat interval | Max repeats |
|---|---|---|---|
| `quiet` | 15 min | 30 min | 3 |
| `normal` | 5 min | 10 min | unlimited |
| `annoying` | 1 min | 2 min | unlimited |

### Example: one-time reminder requiring acknowledgement

```yaml
service: events.fire_event
data:
  event_type: custom_reminder_create
  event_data:
    id: "take_meds_2026_01_15"
    targets:
      - alexey
    message: "Take your medication"
    trigger_at: "2026-01-15T08:00:00+02:00"
    requires_ack: true
    profile: normal
    owner: alexey
    source: morning_routine
```

### Example: recurring reminder, no ack needed

```yaml
service: events.fire_event
data:
  event_type: custom_reminder_create
  event_data:
    id: "water_plants_weekly"
    targets:
      - alexey
      - partner
    message: "Water the plants"
    trigger_at: "2026-01-15T10:00:00+02:00"
    recur_every: "168h"   # 7 days
    requires_ack: false
    source: home_maintenance
```

### Example: reminder with expiry and any-target policy

```yaml
service: events.fire_event
data:
  event_type: custom_reminder_create
  event_data:
    id: "order_groceries_jan"
    targets:
      - alexey
      - partner
    message: "Order groceries before the weekend"
    trigger_at: "2026-01-15T09:00:00+02:00"
    valid_until: "2026-01-17T20:00:00+02:00"
    requires_ack: true
    completion_policy: any_target_ack
    profile: normal
    source: shopping
```

---

## Acknowledging a Reminder

Fire `custom_reminder_ack` with the reminder ID and the user acknowledging.

```yaml
service: events.fire_event
data:
  event_type: custom_reminder_ack
  event_data:
    id: "take_meds_2026_01_15"
    user_id: alexey
```

Acknowledgement is idempotent — firing it again for the same user has no effect.

The MQTT switch entities (`switch.reminder_<id>_<user>`) can also be turned OFF from a dashboard or automation to trigger acknowledgement for that user.

---

## Deleting a Reminder

Fire `custom_reminder_delete` to remove a reminder and clean up its MQTT entities.

```yaml
service: events.fire_event
data:
  event_type: custom_reminder_delete
  event_data:
    id: "take_meds_2026_01_15"
```

Deletion is idempotent.

---

## MQTT Entities

For each active reminder, one switch entity is created per target user:

```
switch.reminder_<reminder_id>_<user_id>
```

Special characters (`/`) in IDs are replaced with `_`.

| State | Meaning |
|---|---|
| `ON` | Reminder is pending for this user |
| `OFF` (command) | User acknowledges the reminder |

Entities are created automatically when a reminder becomes actionable and removed when it completes, is deleted, or expires. After a service restart, entities are restored for all active reminders and stale entities from deleted reminders are cleaned up.

---

## Completion Policies

| Policy | Behaviour |
|---|---|
| `all_targets_ack` (default) | Reminder stays active until every target user acknowledges it. |
| `any_target_ack` | Reminder completes as soon as any one target user acknowledges it. |

---

## Notes

- `id` values must not contain `/` — use `_` or `-` as separators.
- `trigger_at` and `valid_until` must be RFC3339 strings with timezone offset (e.g. `2026-01-15T08:00:00+02:00` or `2026-01-15T06:00:00Z`).
- `recur_every` uses Go duration syntax: `"30m"`, `"1h"`, `"24h"`, `"168h"` (7 days).
- Creating a reminder with an ID that already exists will overwrite it.
