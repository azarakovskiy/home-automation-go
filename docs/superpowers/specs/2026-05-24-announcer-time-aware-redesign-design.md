# Announcer Time-Aware Redesign

## Goal

Replace the `HandleMorning()` / once-per-day dedup model with a single stateless on-demand handler that produces a time-aware, natural-language message. HA decides *when* to ask; the Announcer decides *what to say* based on the current hour.

## Context

The previous design had the Announcer listen to the `DaytimeMode` entity change to fire a morning summary, guarded by a PostgreSQL dedup record. This baked HA-specific entity assumptions and UX scheduling into the Announcer. The redesign removes all of that: HA triggers via MQTT switch (same mechanism already in place for on-demand), and the Announcer shapes the message based on time of day.

---

## Architecture

### Entry points (after)

| Trigger | Handler | Behaviour |
|---|---|---|
| MQTT switch ON | `HandleOnDemand()` | time-aware, stateless |
| Price sensor entity change | `handlePriceUpdate()` | reactive spike alert, unchanged in structure |

The `DaytimeMode` entity listener registered in `app.go` is removed. The `AnnouncerStateStore` dependency and `AnnouncerRepo` are removed from the Announcer entirely.

### Time bucketing

`HandleOnDemand()` reads `a.now().Hour()`:

- **Hour < 11** → morning summary: full day ahead
- **Hour ≥ 11** → afternoon brief: remaining window from now to midnight

The cutoff (11) is hardcoded. No config knob — YAGNI.

---

## Message Formatting

### Separation of concerns

The Announcer gathers a typed context struct and passes it to a `MessageFormatter` interface. The default implementation produces natural-language output. When an LLM persona replaces it later, only the formatter changes.

```go
type AnnouncePeriod int

const (
    PeriodMorning AnnouncePeriod = iota
    PeriodAfternoon
)

type AnnounceContext struct {
    Period       AnnouncePeriod
    CurrentLevel pricing.PriceLevel
    Summary      pricing.IndexSummary // scoped to remaining window (now → midnight)
}

type MessageFormatter interface {
    Format(AnnounceContext) string
}
```

The formatter lives in `formatter.go` inside the `priceannouncer` package.

### Message style

Natural, qualitative language. Times use clock references ("after 5", "until midnight"), not ranges like "17:00–20:00". Price levels use adjectives ("cheap", "expensive", "really expensive"), not ct/kWh. Numbers appear only when they aid meaning. Conjunctions ("but", "keep in mind", "then") connect clauses.

**Morning examples:**

> It's a good day price-wise — cheapest in the early hours until 6, then moderate most of the day. Prices spike sharply after 5 in the afternoon, so best to get things done before that.

> Prices are flat today — nothing dramatic either way. Current price is moderate.

**Afternoon examples:**

> It's cheap right now and stays that way until 5 — good time to act. After that prices spike and don't settle until late tonight.

> Prices are high right now. It gets cheaper after 9 tonight if you can wait.

> Nothing noteworthy ahead — prices are moderate for the rest of the day.

### Content rules

**Morning** (uses `Summary` over full remaining day):
1. Lead with overall character of the day (cheap/expensive/flat)
2. Call out cheap window if present ("cheapest … until X")
3. Call out expensive window if present ("prices spike after X")
4. Call out negative prices if present ("prices go negative tonight")
5. Omit sections with no data — don't say "no cheap windows today"

**Afternoon** (uses `Summary` scoped from now to midnight):
1. Lead with current price level
2. If current is cheap: say how long it lasts and what comes after
3. If current is expensive: say when it improves
4. If current is average: describe what's notable ahead (cheap/expensive), or say "nothing dramatic"
5. Same rules: omit empty sections

---

## Reactive Alert (unchanged in behaviour, dedup added)

`handlePriceUpdate()` retains its existing logic — detect the first qualifying extreme run (spike × median for ≥ `MinExtremeDuration`) and send an alert.

**New: in-memory dedup.** A `lastAlertedRunFrom time.Time` field is added to the `Announcer` struct. Before sending, check if the detected run's `From` equals `lastAlertedRunFrom` — if so, skip. After sending, record it. This prevents re-alerting on every sensor update during an ongoing spike.

On restart, at most one duplicate alert per extreme run may fire — acceptable given restart frequency.

---

## What is removed

- `HandleMorning()` public method
- `AnnouncerStateStore` interface
- `AnnouncerConfig` has no morning-related fields (already clean after previous session)
- `DaytimeMode` entity listener in `app.go`
- `AnnouncerRepo` construction in `app.go` / `buildComponents`
- `postgres.NewAnnouncerRepo` call (and the repo itself if unused elsewhere)
- `TestAnnouncer_MorningSummary_*` tests (all five)

## What is added

- `AnnounceContext` struct and `AnnouncePeriod` type
- `MessageFormatter` interface and default `naturalLanguageFormatter` implementation in `formatter.go`
- `lastAlertedRunFrom time.Time` field on `Announcer`
- Tests: `TestAnnouncer_OnDemand_MorningFormat`, `TestAnnouncer_OnDemand_AfternoonFormat`, `TestAnnouncer_Reactive_NoDuplicateAlert`

---

## Files affected

| File | Change |
|---|---|
| `internal/domain/devices/priceannouncer/announcer.go` | Remove `HandleMorning`, stateless `HandleOnDemand`, add `lastAlertedRunFrom`, wire formatter |
| `internal/domain/devices/priceannouncer/formatter.go` | New — `AnnounceContext`, `MessageFormatter`, default implementation |
| `internal/domain/devices/priceannouncer/announcer_test.go` | Remove morning tests, add time-bucket and dedup tests |
| `internal/app.go` | Remove `DaytimeMode` listener, remove `AnnouncerRepo` construction, remove `AnnouncerStateStore` arg |
| `internal/tech/postgres/announcer_repo.go` | Delete (no remaining callers) |
