# Pricing Redesign

**Date:** 2026-05-22
**Scope:** `internal/domain/pricing`, `internal/domain/optimizer`, new `Announcer` component
**Goal:** Fix two bugs (optimizer skips active slot; announcements fire late), remove histogram machinery (~200 lines), and replace ad-hoc announcement logic with a focused `Announcer` component. Zero behaviour change outside the bugs.

---

## Problems

### Bug 1 — Optimizer skips the currently-active slot

`findBestWindow` skips any slot where `From.Before(now)`. A slot active at 14:30 has `From=14:00`, so it is excluded as a start candidate. If that slot is the cheapest, the optimizer picks a later one and delays when starting immediately would be optimal.

Same condition in `findFirstValidSlot` causes `calculateSavings` and `createImmediateResult` to use the *next* slot's price as the "start now" baseline, making savings comparisons wrong.

### Bug 2 — Announcements fire late and only reflect the current window

`buildAnnouncementWindow` finds the level of the first non-past slot and extends it forward until the level changes. It never looks ahead. At 22:00 in an expensive slot with cheap starting at midnight, it announces "expensive" and the cheap alert waits until midnight — where the 2 h cooldown may delay it further. The result is late or missing "cheap prices ahead" notices.

### Complexity — Histogram

~200 lines of bucket/histogram machinery in `levels.go` and `service.go` accumulates historical price data to improve threshold accuracy. The same questions (is this price cheap or expensive?) are better answered day-relative: percentile rank against today's available 24 h of slots. No history needed.

---

## Design

### 1. `PriceIndex` — pure value type

New type that owns all pricing queries. IO-free; constructed from a `[]PriceSlot` snapshot.

```go
type PriceIndex struct {
    slots          []PriceSlot
    cheapThreshold float64
    expThreshold   float64
}

func NewPriceIndex(slots []PriceSlot) PriceIndex
func (idx PriceIndex) IsEmpty() bool
func (idx PriceIndex) SlotAt(t time.Time) (PriceSlot, bool)
func (idx PriceIndex) Level(slot PriceSlot) PriceLevel
func (idx PriceIndex) FindCheapestWindow(duration time.Duration, from, deadline time.Time) ([]PriceSlot, bool)
func (idx PriceIndex) FindCheapestSlots(n int, from, deadline time.Time) []PriceSlot
func (idx PriceIndex) HasNegativePrices(from, deadline time.Time) bool
func (idx PriceIndex) Summary(from, deadline time.Time) IndexSummary
```

Thresholds computed once at construction via `ComputeThresholdsFromPrices` (percentile of current day's slots). `ComputeThresholdsFromPrices` stays in `levels.go`.

Slot duration is not assumed — it is derived from the data (`slot.Till.Sub(slot.From)`). The index works correctly whether slots are 1 h or 15 min. No constant or field encodes a fixed slot size; any logic that needs a "how many slots cover N minutes" calculation derives it from the actual slot boundaries.

`FindCheapestWindow` returns the lowest-cost consecutive block of the requested duration. `FindCheapestSlots` returns the N cheapest non-consecutive slots (for opportunistic chargers). `Summary` returns a human-readable overview (cheap windows, expensive peaks, negative price periods) used by the Announcer.

### 2. `Service` — cache + HA adapter

Holds the current `PriceIndex`, updated on each price event.

```go
type Service struct {
    state ga.State
    mu    sync.RWMutex
    index PriceIndex
    now   func() time.Time
}

func (s *Service) UpdateIndex(slots []PriceSlot)
func (s *Service) CurrentIndex() (PriceIndex, error)  // error if empty
func (s *Service) GetPriceSlots() ([]PriceSlot, error) // existing callers; delegates to CurrentIndex
```

`sync.RWMutex` is the right primitive here: writes happen once or twice per day (price release), reads are concurrent and frequent (dishwasher, announcer, dashboard). Readers take a snapshot copy and release the lock immediately — no long-held locks.

`UpdateIndex` is called from `applyParsedPrices` (already in master). Announcement logic is not called from here.

**Night/away mode abstraction.** Currently read from HA entities; may migrate to Go-owned state later. Defined behind an interface from the start:

```go
type ModeProvider interface {
    IsNight() (bool, error)
    IsAway() (bool, error)
}
```

`Service` does not use `ModeProvider` — it is passed to `Announcer` only.

**Transport abstraction.** All HA coupling (entity listeners, custom events, input_text helpers) lives in adapters. `Service` and all business logic never import HA or MQTT packages directly. When communication migrates to MQTT, only the adapter layer changes.

### 3. `Announcer` — independent component

Owns all price announcement logic. Consumes `*pricing.Service`; does not live inside `Service`.

```go
type Announcer struct {
    service      *pricing.Service
    modes        ModeProvider
    notification NotificationSender
    db           AnnouncerStateStore
    cfg          AnnouncerConfig
}

type AnnouncerConfig struct {
    SpikeMultiplier    float64       // e.g. 3.0× day median triggers reactive alert
    MinExtremeDuration time.Duration // consecutive extreme slots must sum to at least this; e.g. 1h
    // other thresholds as needed
}

type AnnouncerStateStore interface {
    LastAnnouncedDate(ctx context.Context) (time.Time, error)
    SetLastAnnouncedDate(ctx context.Context, t time.Time) error
}
```

Three trigger paths:

**Morning context trigger.** Listens to a HA entity state change that signals "morning has started" (coffee machine drawing power, TV on, etc.). Which entity is wired at registration time — the `Announcer` logic is entity-agnostic. Fires at most once per calendar day: checks `AnnouncerStateStore` for today's date on entry; if already announced, skips. On fire, calls `index.Summary(now, midnight)`, formats a human-readable day overview (cheap windows, expensive peaks, negative prices), sends via `NotificationSender`. Persists the announcement date to PostgreSQL so a service restart does not re-trigger.

Night/away suppression: checks `modes.IsNight()` and `modes.IsAway()` before firing.

**On-demand trigger.** Listens to a HA event (fired by a dashboard button, HA script, or Google Assistant routine via HA integration). No cooldown — user-initiated. Same summary as morning trigger. No deduplication gate. Night/away suppression applies.

**Reactive alert.** Also listens to the price sensor change event (same trigger as `applyParsedPrices`). After the index updates, checks whether the incoming day contains a consecutive run of extreme slots — negative prices or prices above `cfg.SpikeMultiplier × day median` — whose total duration meets or exceeds `cfg.MinExtremeDuration` (e.g. 1 h). A single outlier 15-min slot does not fire. If the threshold is met, fires one alert describing the window. Night/away suppression applies. No per-day gate — an extreme mid-day event should fire even if a morning summary already ran.

### 4. Optimizer bug fixes

Two one-line fixes in `internal/domain/optimizer/optimizer.go`.

**`findBestWindow` (line 160):**

```go
// Before — skips currently-active slot
if startSlot.From.Before(now) { continue }

// After — only skips fully past slots
if startSlot.Till.Before(now) { continue }
```

When the active slot is selected as start candidate, `StartTime` is clamped to `now` and `EndTime` recalculated as `now + cycleDuration`, so the result does not report a past start time and the deadline check uses the real remaining window.

**`findFirstValidSlot` (line 226):**

```go
// Before — returns first slot whose From >= now (skips active slot)
if !slot.From.Before(now) { return i }

// After — returns first slot not yet fully past
if !slot.Till.Before(now) { return i }
```

Fixes both `createImmediateResult` and `calculateSavings`: the "start now" cost baseline uses the currently-active slot's price, not the next slot.

---

## What disappears

| Location | Removed |
|---|---|
| `levels.go` | `Bucket`, `RoundPriceToBucket`, `BuildBucketsFromHistogram`, `PercentileFromBuckets`, `PriceBucketSize`, `MinSamplesForHistogram` |
| `service.go` | Histogram fields on `Service`, `ingestHistogram`, `thresholdsFromHistogram`, `classifyPrice`, `maybeAnnounce`, `canAnnounce`, HA input_text read/write |
| `window.go` | `buildAnnouncementWindow`; file deleted if nothing remains |

`ComputeThresholdsFromPrices` stays. `applyParsedPrices`, `updateFromAttributes`, `refreshFromState` stay unchanged.

Net change: pricing package −~200 lines, +~80. Announcement logic moves out of `Service` entirely. No HA input_text helpers. No accumulated historical state — pricing is ephemeral, rebuilt on each price update.

---

## Non-goals

- No changes to `PriceSlot`, `PriceLevel`, or public `Service` method signatures consumed by the dishwasher.
- No changes to dishwasher scheduling logic beyond the optimizer fix.
- No new device profiles or scheduling strategies.
- No MQTT migration in this scope — interfaces are designed to accommodate it, but the migration itself is separate work.
- No PostgreSQL schema beyond the single `AnnouncerStateStore` row.

---

## Testing

Existing `service_test.go` serves as regression harness for `Service`. `PriceIndex` gets its own unit tests (pure functions, no mocks needed). Optimizer fixes get targeted tests: a test where `now` falls mid-slot verifies the active slot is included as a candidate and used as the cost baseline. `Announcer` tests use a fake `AnnouncerStateStore` and fake `NotificationSender`.
