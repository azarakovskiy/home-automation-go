# Reminders Feature Design

**Date:** 2026-05-23  
**Status:** Approved

## Overview

A reminder service that creates, schedules, and delivers time-based or event-triggered reminders to household members. Reminders escalate in urgency over time until acknowledged. The service owns scheduling and lifecycle; delivery adapters own channel routing.

---

## Use Cases

1. **Recurring personal reminder** — medication, daily task. Fires repeatedly on a schedule, escalating until acknowledged. Lives until removed.
2. **Event-triggered one-shot** — dishwasher finished, laundry done. Created automatically by another component. Escalates until acknowledged, then gone.
3. **Expiring soft reminder** — call someone, low priority. Fires a few times; if ignored past a deadline, expires automatically. Ack also removes it.
4. **Per-person reminder** — targets a specific user; that user (or anyone via god-mode) must acknowledge it.
5. **Household reminder** — targets all household members; first ack from anyone completes it.
6. **God-mode ack** — any user can acknowledge on behalf of any other user (e.g. spouse is away). This is a UI concern; the service treats it as a normal ack for the target user.

---

## Domain Model

### Core Types

```
Reminder
  ID            string
  Targets       []string          // user IDs; at least one required
  Schedule      Schedule
  Policy        DeliveryPolicy
  State         State
  Meta          Metadata
  Acks          []UserAck
```

```
Schedule
  Kind          ScheduleKind      // once | recurring
  TriggerAt     time.Time
  NextRunAt     *time.Time
  RecurEvery    *time.Duration    // required for recurring
  ValidUntil    *time.Time        // optional expiration
```

```
DeliveryPolicy
  RequiresAck   bool
  Profile       Profile           // quiet | normal | annoying
```

```
State
  CreatedAt     time.Time
  UpdatedAt     time.Time
  LastFiredAt   *time.Time
  FireCount     int               // incremented by Trigger(); used for MaxRepeats enforcement
```

```
Metadata
  Source   string   // "manual" | component name
  Owner    string   // creating user or system
  Message  string
```

```
UserAck
  UserID   string
  AckedAt  time.Time
```

### Escalation Profiles

Each profile defines how quickly reminders repeat and when they stop on their own.

| Profile  | InitialDelay | RepeatInterval              | MaxRepeats |
|----------|-------------|-----------------------------|------------|
| quiet    | 30 min      | 1 hour (fixed)              | 3          |
| normal   | 15 min      | 15 min (fixed)              | unlimited  |
| annoying | 15 min      | decreases each fire, min 5 min | unlimited  |

`MaxRepeats` is enforced by `Trigger()`: when `FireCount >= MaxRepeats` and the profile has a limit, the reminder expires rather than scheduling another repeat.

### Completion

Single completion model: the first ack from any target completes the reminder. There is no "all targets must ack" policy. Completed reminders are deleted from storage permanently.

### Lifecycle Methods

- `New(cmd CreateCommand) (Reminder, error)` — validates and constructs
- `IsDue(now time.Time) bool`
- `Trigger(now time.Time)` — advances state, computes next `NextRunAt`, enforces `MaxRepeats`
- `Acknowledge(targetUserID string) error` — records ack for `targetUserID`; errors if `targetUserID` is not in Targets. The caller identity is irrelevant — god-mode ack is simply calling this with another user's ID.
- `IsComplete() bool` — true if any target has acked
- `IsExpired(now time.Time) bool`

### Errors

- `ErrNotFound`
- `ErrNoTargets`
- `ErrInvalidSchedule`
- `ErrNotTarget`
- `ErrNotActive`

---

## Repository Interface

Defined consumer-side in the `reminders` package.

```go
type Repository interface {
    Save(ctx context.Context, r Reminder) error
    GetByID(ctx context.Context, id ReminderID) (Reminder, error)
    ListActive(ctx context.Context) ([]Reminder, error)
    ListDueBefore(ctx context.Context, t time.Time) ([]Reminder, error)
    Remove(ctx context.Context, id ReminderID) error
}
```

All stored reminders are active. Completed, expired, and deleted reminders are hard-deleted via `Remove` — no status column, no history. `Save` is only called for live state updates (ack recorded, NextRunAt advanced).

---

## Notifier Interface

Defined consumer-side in the `reminders` package. Fire-and-forget; no delivery confirmation.

```go
type Notification struct {
    ID   string
    To   []string
    Body string
}

type Notifier interface {
    Notify(ctx context.Context, n Notification) error
}
```

The manager calls `Notify` when a reminder fires. Delivery is fire-and-forget and instant — no queuing, no cancellation. The notifier implementation owns channel selection, DND handling, and all delivery specifics.

---

## Manager

```go
type Manager struct {
    repo     Repository
    notifier Notifier
}
```

### Operations

**Create(cmd CreateCommand) (Reminder, error)**  
Validates, constructs via `New()`, persists.

**Ack(ctx, reminderID, targetUserID string) error**  
Loads, calls `rem.Acknowledge(targetUserID)`, checks `IsComplete()`. If complete: removes from repo. Otherwise saves updated acks.

**Delete(ctx, reminderID string) error**  
Removes from repo.

**Tick(ctx, now time.Time) error**  
Lists due reminders. For each:
1. If `IsExpired(now)`: remove from repo → skip
2. Call `rem.Trigger(now)` — advances `FireCount`, computes next `NextRunAt`; if `MaxRepeats` reached, expires instead
3. If expired after Trigger: remove from repo → skip
4. Call `notifier.Notify`
5. Save

---

## Adapters

### Notifier

`reminders.Notifier` is satisfied by the existing `NotificationService` (`internal/tech/homeassistant/notifications`) — either directly if its signature aligns, or via a small adapter in `app.go`. The implementation detail of how it reaches users (MQTT topic, push, voice) is entirely internal to the adapter.

### Event Handler

A separate component that subscribes to MQTT command topics and calls Manager methods:

| Topic | Action |
|-------|--------|
| `{prefix}/reminders/create` | `Manager.Create` |
| `{prefix}/reminders/ack` | `Manager.Ack` |
| `{prefix}/reminders/delete` | `Manager.Delete` |

Uses the existing `runtimeTransport` (Paho MQTT, already wired in the app). Payloads are JSON.

Also owns the 1-minute tick: calls `Manager.Tick(ctx, now)` on schedule. Active reminders surviving a restart are picked up naturally on the next tick — no explicit restore needed.

---

## Storage

PostgreSQL. Three tables unchanged in shape:

- `reminders` — flattened fields; `requires_ack` stored as native `BOOLEAN` (not `int64`)
- `reminder_targets`
- `reminder_acks`

Completed and expired reminders are hard-deleted. No history table.

---

## What Changes From Current Implementation

| Area | Change |
|------|--------|
| Completion policy | Drop `all_targets_ack`; remove enum value and all related logic |
| `MaxRepeats` | Enforce in `Trigger()`; reminder auto-expires when limit reached |
| `Repository` | Replace `Delete` with `Remove(id)` for hard-deletion; drop status column |
| `FireCount` | Add to `State` and DB schema; incremented by `Trigger()` |
| `requires_ack` column | Change from `int64` to native PostgreSQL `BOOLEAN` |
| `ReminderStatus` enum | Remove entirely — all stored reminders are active |
| Manager | Inject `Notifier`; `Tick` calls `notifier.Notify` on fire; no Action return values |
| Notifier | `NotificationService` satisfies `reminders.Notifier` directly or via small `app.go` adapter |
| Event handler | MQTT command subscriber (replaces HA custom events); uses existing Paho transport |
| Old specs | Remove `specs/reminders/` after this spec is committed |

---

## Out of Scope

- Reminder history / delivery log
- Snooze / dismiss separate from acknowledgement
- Delivery confirmation / async feedback from adapter
- DND delay at manager level (adapter handles quiet delivery)
- Multiple transport adapters simultaneously
