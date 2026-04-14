# Reminders Feature Specification

Agents: this file is read-only unless explicitly asked to change. Suggestions of improvements are still welcome.

## Purpose

This file describes how the final reminders feature must behave. It is the product and behavior spec, not the task tracker. 

## Goal

Implement a reminders system where Go is the source of truth, SQLite persists reminder state, Home Assistant sends commands through custom events, and all Home Assistant-facing reminder entities are exposed through runtime MQTT entities only.

## Core Principles

- Reminder state lives in Go and SQLite, not in Home Assistant helpers.
- Home Assistant is a controller and UI surface, not the source of truth.
- Reminder behavior belongs in the domain layer.
- MQTT entity creation and cleanup belong in the Home Assistant adapter layer.
- Reminders are managed via custom events from HASS; current state is exposed via MQTT entities.

## Shared Scope

The feature supports:

- one-time reminders
- recurring reminders
- reminders that require acknowledgement
- escalating repeats for acknowledgement-required reminders
- per-user targeting
- per-user acknowledgement
- priority/profile-driven reminder behavior
- expiration / valid-until cutoff
- source and owner metadata
- runtime restoration after service restart

The feature does not require Home Assistant config changes as part of this work. Existing custom events are assumed to work.

## Domain Concepts

### Reminder

One reminder is a single aggregate with nested configuration and runtime state.

Expected nested parts:

- `Schedule`
- `DeliveryPolicy`
- `State`
- `Metadata`

### Target Users

The reminder may target any amount (1+) of users. 

### Per-User Acknowledgement

Acknowledgement is tracked per user. A reminder visible to multiple users must not collapse into one global boolean acknowledgement flag.

### Completion Policy

Each reminder carries a completion policy that controls when it is considered fully acknowledged.

Two policies are supported:

**`all_targets_ack`** (default)

- the reminder remains active until every target user has acknowledged it
- if the reminder targets one user, one acknowledgement completes it
- partial acknowledgement keeps the reminder alive for remaining users

**`any_target_ack`**

- the reminder is considered complete as soon as any one target user acknowledges it
- after the first acknowledgement the reminder transitions to completed status
- acknowledgements from other users are still recorded but do not change the completion outcome
- per-user acknowledgement state is still tracked for audit purposes

The default completion policy is `all_targets_ack`.

### Priority / Profile

Each reminder has a profile:

- `quiet`
- `normal`
- `annoying`

Profiles influence escalation behavior.

### Quiet Hours

Quiet hours are per-user do-not-disturb windows. Quiet hours affect delivery timing, not reminder ownership or persistence.

Expected behavior:

- `quiet`: defer delivery until quiet hours end
- `normal`: defer unless the reminder is already overdue by configured rules
- `annoying`: allowed to continue or use a shorter defer strategy

### Expiration

Each reminder may define `valid_until`.

Behavior:

- if `valid_until` is absent, the reminder remains valid until completed or deleted
- if `valid_until` is present and the current time passes it, the reminder expires and must stop producing new deliveries
- expired reminders must be cleaned up from MQTT runtime entities

### Metadata

Each reminder stores:

- `source`: where the reminder came from, for example an automation or subsystem name
- `owner`: who created or owns the reminder logically

Metadata is informational and filterable. It must not change reminder lifecycle semantics.

## Transport Boundaries

### Command Input

Commands arrive from Home Assistant custom events.

Required commands:

- create reminder
- acknowledge reminder
- delete reminder

The service listens for these events similarly to the existing notification service pattern.

### UI / Projection Output

All Home Assistant-facing reminder entities must be exposed through runtime MQTT entities only.

Implications:

- no static custom entities for reminders
- no helper-backed reminder state
- no Home Assistant-owned source of truth

Runtime MQTT projection is a view of reminder state, not the reminder state itself.

## MQTT Projection Rules

For reminders that are currently actionable in Home Assistant:

- create runtime MQTT entities using stable keys derived from reminder ID and user ID where needed
- expose reminder text/state in a sensor-like entity
- expose acknowledgement controls through runtime MQTT entities

When a reminder is no longer actionable:

- remove all associated runtime MQTT entities

After restart:

- restore runtime entities for still-active reminders
- reconcile away stale reminder entities that are no longer backed by SQLite state

## Observable Behavior

- A one-time reminder fires once at its scheduled time.
- A recurring reminder computes and stores the next run after each trigger until completed, deleted, or expired.
- A reminder with `requires_ack=true` repeats until its completion policy is satisfied or it expires.
- A reminder with multiple targets keeps per-user acknowledgement state regardless of completion policy.
- A reminder with `all_targets_ack` (default) is complete only when every target user has acknowledged it.
- A reminder with `any_target_ack` is complete as soon as any one target user acknowledges it.
- A reminder past `valid_until` stops triggering and is removed from active projection.
- Deleting a reminder removes it from active behavior and cleans up runtime MQTT entities.
- Quiet hours are evaluated per user and may defer delivery for one user while keeping the reminder active for another.

## Acceptance Criteria

- creating a reminder through the custom create event persists it in SQLite
- acknowledging a reminder for one target user updates only that user acknowledgement state
- a multi-user reminder with `all_targets_ack` remains active until all target users acknowledge it
- a multi-user reminder with `any_target_ack` completes as soon as one target user acknowledges it
- per-user acknowledgement state is recorded even for `any_target_ack` reminders after completion
- a reminder targeted to two users may be deferred for one user and still active for the other during quiet hours
- runtime MQTT entities are restored after restart for active reminders
- stale reminder runtime MQTT entities are reconciled away after restart
- `make test` passes with table-driven coverage for escalation math, acknowledgement logic, quiet-hours behavior, expiration, and manager tick behavior

## Done When

- the system can create, persist, restore, trigger, acknowledge, delete, and expire reminders with per-user acknowledgement
- the system applies per-user quiet-hours behavior without mutating reminder ownership or acknowledgement history
- Home Assistant-facing reminder entities exist only through runtime MQTT entities
- no reminder state depends on Home Assistant helper entities

## Edge Cases

- creating a reminder with no targets is invalid
- acknowledging a reminder for a user not in the target set is invalid
- acknowledging an already-acknowledged reminder for that same user is idempotent
- with `any_target_ack`, acknowledging after the reminder is already complete is idempotent
- deleting an already-deleted reminder is idempotent
- expired reminders must not schedule a new next run
- reminders due during downtime must recover deterministically on restart
- a reminder visible to multiple users must not disappear from MQTT projection until completion policy is satisfied

## Non-Goals

- Home Assistant YAML cleanup
- dashboard design changes
- historical analytics
- arbitrary multi-channel delivery abstraction
