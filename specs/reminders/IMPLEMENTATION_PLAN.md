# Reminders Implementation Plan

Status: Active.

This file is the execution tracker for the reminders feature. Keep steps atomic, update statuses as work progresses, and use this file to describe how implementation reaches the final specification.

## Status Legend

- `PENDING`: not started
- `IN PROGRESS`: current active step
- `DONE`: completed and reflected in code
- `BLOCKED`: cannot proceed without a decision or prerequisite

## Current Step

`DONE`: V1 Core complete. Slice 8 (V1.1 Quiet Hours) is next.

## Delivery Slices

## Slice 0: Spec Scaffold

- `DONE` Create `specs/reminders/`.
- `DONE` Add a read-mostly north-star feature specification for humans.
- `DONE` Add this implementation plan with atomic steps and status tracking.
- `DONE` Add agent instructions for executing the plan and maintaining status.

## Release Plan

The specification describes the final feature. Versioning and incremental delivery live here.

- V1: minimal core, persistence, manager, custom-event handling, MQTT projection, per-user acknowledgement, both completion policies, expiration, metadata
- V1.1: quiet hours / per-user do-not-disturb
- V1.2: snooze, dismiss, history, richer policy work

## Slice 1: V1 Core Domain

- `DONE` V1-01 Replace the stub in `internal/domain/reminders/domain.go` with real domain types:
  - `Reminder`
  - `Schedule`
  - `DeliveryPolicy`
  - `State`
  - `Metadata`
  - `UserAck`
  - value types for profile, schedule kind, status, and completion policy (`all_targets_ack`, `any_target_ack`)
- `DONE` V1-02 Remove `internal/domain/reminders/repository/repository.go` if it exists (clean up any stub).
- `DONE` V1-03 Define one repository interface in the main reminders package with the methods needed by the manager.
- `DONE` V1-04 Define per-user acknowledgement types in the domain model.
- `DONE` V1-05 Implement pure domain validation rules:
  - at least one target user
  - valid schedule shape
  - valid completion policy (`all_targets_ack` or `any_target_ack`)
  - valid acknowledgement actor
- `DONE` V1-06 Implement pure domain lifecycle methods:
  - due check
  - trigger transition
  - per-user acknowledgement (records ack regardless of policy)
  - `IsComplete()` respects completion policy: `all_targets_ack` requires all, `any_target_ack` requires one
  - delete
  - expire
  - next-run calculation
- `DONE` V1-07 Implement escalation policy presets for `quiet`, `normal`, and `annoying`.
- `DONE` V1-08 Add table-driven tests for the domain model:
  - once reminders
  - recurring reminders
  - per-user acknowledgement
  - `all_targets_ack`: completes only when all users ack
  - `any_target_ack`: completes on first ack, subsequent acks are idempotent
  - expiration
  - escalation presets

## Slice 2: SQLite Persistence

- `DONE` V1-09 Add `modernc.org/sqlite` and `github.com/golang-migrate/migrate/v4` to `go.mod`.
- `DONE` V1-10 Add `sqlc.yaml` at repo root (sqlite engine, schema from migrations, queries from sql/, output to `internal/tech/sqlite/sqlc/`).
- `DONE` V1-11 Add `make sqlc` target to Makefile.
- `DONE` V1-12 Create `internal/tech/sqlite/migrations/000001_reminders.up.sql` with three tables:
  - `reminders` (flattened config + runtime state, completion_policy column)
  - `reminder_targets`
  - `reminder_acks`
- `DONE` V1-13 Create `internal/tech/sqlite/migrations/000001_reminders.down.sql`.
- `DONE` V1-14 Create `internal/tech/sqlite/sql/reminders.sql` with sqlc-annotated queries.
- `DONE` V1-15 Run `make sqlc` to generate `internal/tech/sqlite/sqlc/`.
- `DONE` V1-16 Create `internal/tech/sqlite/database.go` that opens the DB and applies golang-migrate migrations embedded in the binary (`embed.FS` + `iofs` source, `WithInstance` for DB driver reuse). Conversion helpers extracted to `conversion.go`.
- `DONE` V1-17 Build `internal/tech/sqlite/reminders_repo.go` implementing the `reminders.Repository` interface using generated sqlc queries.
- `DONE` V1-18 Add SQLite round-trip tests in `internal/tech/sqlite/reminders_repo_test.go`.
- `DONE` V1-19 Add tests for multi-target reminders and per-user acknowledgements.
- `DONE` V1-20 Add tests for expiration filtering and due reminder queries.

## Slice 3: Reminder Manager

- `DONE` V1-21 Add `manager.go` in `internal/domain/reminders`.
- `DONE` V1-22 Implement `Create`.
- `DONE` V1-23 Implement `Ack`.
- `DONE` V1-24 Implement `Delete`.
- `DONE` V1-25 Implement `Restore`.
- `DONE` V1-26 Implement `Tick(now)`.
- `DONE` V1-27 Define manager result actions for the adapter layer:
  - show or refresh projection
  - remove projection
  - no-op
- `DONE` V1-28 Add repository mock generation for manager tests.
- `DONE` V1-29 Add table-driven manager tests:
  - create path
  - delete path
  - single-user ack
  - multi-user partial ack (`all_targets_ack`)
  - multi-user completion (`all_targets_ack`)
  - `any_target_ack` completion on first ack
  - recurring due reminders
  - expired reminders
  - restore behavior

## Slice 4: Home Assistant Adapter

- `DONE` V1-30 Add `internal/tech/homeassistant/devices/reminders/`.
- `DONE` V1-31 Define event DTOs for create, ack, and delete requests (`events.go`).
- `DONE` V1-32 Add reminder custom event constants in `internal/tech/homeassistant/entities/custom_entities.go`.
- `DONE` V1-33 Implement a reminders component constructor with required dependencies:
  - base component
  - runtime MQTT entity service
  - reminder manager
- `DONE` V1-34 Register custom event listeners using the existing typed event handler pattern.
- `DONE` V1-35 Register one periodic tick interval (1-minute).
- `DONE` V1-36 Map create event payloads into the domain create command.
- `DONE` V1-37 Map ack event payloads into per-user acknowledgement commands.
- `DONE` V1-38 Map delete event payloads into delete commands.

## Slice 5: MQTT Runtime Projection

- `DONE` V1-39 Define stable runtime MQTT key conventions:
  - key format: `reminder_{reminderID}_{userID}` (both parts slash-sanitized)
  - per-user Switch entity for each target
- `DONE` V1-40 Implement projection creation for actionable reminders (`showProjection`).
- `DONE` V1-41 Implement projection refresh when reminder state changes (re-calling `showProjection` is idempotent; `runtime.declare` is upsert and `OnCommand` overwrites the handler).
- `DONE` V1-42 Implement runtime MQTT acknowledgement controls (per-user Switch entity set ON when active).
- `DONE` V1-43 Wire MQTT command handlers to per-user acknowledgement (Switch OFF command → `Manager.Ack`).
- `DONE` V1-44 Implement projection removal for:
  - fully acknowledged reminders (all_targets_ack: all acked; any_target_ack: first ack)
  - deleted reminders
  - expired reminders
- `DONE` V1-45 Implement startup restore of active projections (`Component.Restore`).
- `DONE` V1-46 Call runtime reconcile so stale retained reminder entities are removed (called inside `Restore`).
- `DONE` V1-47 Add adapter tests for MQTT entity creation, ack command handling, cleanup, and restore.

## Slice 6: App Wiring

- `DONE` V1-48 Add `DatabaseConfig` to `internal/config/config.go` with `Path` field (`SQLITE_PATH` env var, default `./reminders.db`). Runtime registry path not needed.
- `DONE` V1-49 Initialize SQLite reminders repository during app startup.
- `DONE` V1-50 Initialize the reminders manager.
- `DONE` V1-51 Initialize and register the Home Assistant reminders component in `internal/app.go`.
- `DONE` V1-52 Verify runtime entity restore path is invoked on startup.

## Slice 7: V1 Hardening

- `DONE` V1-53 Run and fix unit tests.
- `DONE` V1-54 Run `make test`.
- `DONE` V1-55 Review for leaked Home Assistant-specific concerns inside the domain.
- `DONE` V1-56 Review for missing error wrapping and contextual logs. Fixed: `hydrateList` now wraps `loadAggregate` errors with reminder ID context.
- `DONE` V1-57 Review that no reminder state is persisted in Home Assistant helpers.

## Slice 8: V1.1 Quiet Hours

- `PENDING` V1.1-01 Add a quiet-hours resolver seam.
- `PENDING` V1.1-02 Define per-user quiet-hours lookup behavior.
- `PENDING` V1.1-03 Extend reminder delivery policy with quiet-hours handling flags if not already present.
- `PENDING` V1.1-04 Implement quiet-hours-aware next delivery calculation.
- `PENDING` V1.1-05 Implement per-profile behavior during quiet hours:
  - `quiet`
  - `normal`
  - `annoying`
- `PENDING` V1.1-06 Add tests for mixed-user quiet-hours scenarios.
- `PENDING` V1.1-07 Update projection logic if delivery becomes user-deferred.

## Slice 9: V1.2 Candidate Work

- `PENDING` V1.2-01 Add snooze if product demand is real.
- `PENDING` V1.2-02 Add dismiss separate from acknowledgement if product demand is real.
- `PENDING` V1.2-03 Add reminder history / delivery log only if explicitly requested.
- `PENDING` V1.2-04 Reassess channel abstraction only if more outputs are added beyond MQTT runtime entities.

## Suggestions And Open Questions

Use this section for proposed improvements that are not yet approved for implementation. Do not silently widen scope.

- Consider whether runtime projection should be one shared reminder view or one per-target reminder view in V1. The schema supports either, but the UI behavior should stay consistent.
- If the same reminder may be acknowledged from MQTT and custom events concurrently, confirm whether last-write-wins semantics are sufficient or whether stronger duplicate suppression is needed.
