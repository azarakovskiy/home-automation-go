# Reminders MQTT API

The reminders subsystem is controlled entirely via MQTT. All topics are prefixed with the app's configured MQTT prefix (e.g. `homeautomation`).

## Topics

| Topic | Direction | Description |
|---|---|---|
| `{prefix}/reminders/create` | publish → app | Create a new reminder |
| `{prefix}/reminders/ack` | publish → app | Acknowledge a reminder |
| `{prefix}/reminders/delete` | publish → app | Delete a reminder |

## Create

**Topic:** `{prefix}/reminders/create`

```json
{
  "id": "rem-morning-meds",
  "targets": ["alexey"],
  "message": "Take your vitamins",
  "owner": "alexey",
  "source": "ha-automation",
  "trigger_at": "2026-05-25T08:00:00Z",
  "recur_every": "",
  "valid_until": "2026-05-25T10:00:00Z",
  "requires_ack": true,
  "profile": "normal"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique reminder ID (caller chooses) |
| `targets` | string[] | yes | List of user IDs to notify |
| `message` | string | yes | Notification text |
| `owner` | string | no | User who created the reminder |
| `source` | string | no | System or automation that created it |
| `trigger_at` | string (RFC3339) | yes | When to fire the reminder |
| `recur_every` | string (Go duration) | no | Repeat interval, e.g. `"1h"`, `"30m"`. Omit for one-shot. |
| `valid_until` | string (RFC3339) | no | Silently discard instead of firing after this time |
| `requires_ack` | bool | no | If true, reminder re-fires until acknowledged |
| `profile` | string | no | Escalation profile: `"quiet"`, `"normal"` (default), `"annoying"` |

### Escalation profiles (requires\_ack only)

| Profile | Initial delay | Repeat interval | Max repeats |
|---|---|---|---|
| `quiet` | 30 min | 1 h | 3 |
| `normal` | 15 min | 15 min | unlimited |
| `annoying` | 15 min | 15 min (decreasing by 2 min each time, min 5 min) | unlimited |

## Ack

**Topic:** `{prefix}/reminders/ack`

```json
{
  "id": "rem-morning-meds",
  "user_id": "alexey"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Reminder ID to acknowledge |
| `user_id` | string | yes | ID of the user acknowledging |

Any target can ack to dismiss the reminder for everyone (any-ack policy).

## Delete

**Topic:** `{prefix}/reminders/delete`

```json
{
  "id": "rem-morning-meds"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Reminder ID to delete |

Deletes the reminder unconditionally, regardless of state.
