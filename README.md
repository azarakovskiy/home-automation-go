# Home Automation (Go)

Event-driven Home Assistant automations written in Go. Components talk to HA purely through entities and custom events so UI automations stay declarative while logic lives here.

---

## Working Locally

1. **Clone & init submodule**
   ```bash
   git clone git@github.com:azarakovskiy/home-automation-go.git
   cd home-automation-go
   git submodule update --init --recursive   # pulls the HA config into ./home-automation
   ```
2. **Create local env**
   ```bash
   cp env.example .env
   # edit HA_URL and HA_AUTH_TOKEN (long-lived token)
   ```
3. **Generate entity constants (whenever HA helpers change)**
   ```bash
   cp gen.yaml.example gen.yaml   # only first time
   # edit with HA URL/token if different from .env
   make generate
   ```
4. **Run**
   ```bash
   make run           # normal mode
   DRY_RUN=true make run   # logs actions without touching devices
   ```

> The `home-automation/` submodule contains the HA YAML (helpers, scripts, dashboards). Edit it there, reload helpers in HA, then rerun `make generate` so `entities/entities.go` stays in sync.

---

## Automations

| Automation | Description | Key Entities |
|------------|-------------|--------------|
| **Dishwasher Scheduler** | Listens for `event.custom_scheduled_start`, optimizes start time vs energy prices, persists state across restarts, and announces savings via TTS. | Input helpers under `input_boolean.kitchen_dishwasher_*`, `input_datetime.kitchen_dishwasher_scheduled_start`, price sensors. |
| **Laptop Charger Optimizer** | Every 15 min finds the cheapest slots in the next 12 h to deliver a 6 h charge. Disables charging when house is away >2 h. | `input_boolean.office_laptop_charge_optimization_auto`, charger switch/sensor entities. |
| **Vacuum Charger Optimizer** | Same algorithm with 1 h budget to keep the robot topped off without peak prices. | `input_boolean.living_room_vacuum_charge_optimization_auto`, vacuum dock switch/sensors. |
| **Reminder Service** | Fully event-driven reminders stored in chunked HA helpers. Supports “normal/annoying/quiet” profiles, repeating or one-time mode, optional speaker & phone targets, and per-user visibility. Persists config/runtime/views through compressed HA helper chunks so reminders survive restarts. | `input_text.home_go_reminders_*_chunk_<n>`, scripts `script.reminders_dev_*` for manual testing. |
| **Notification Relay** | Device components raise `event.custom_notify` with a ready-to-speak sentence; HA automations fan that out to TTS or mobile push. | `event.custom_notify`, `script.[speaker]_announce_*`. |

---

## Event Catalog

### Dishwasher Scheduling
| Event | Payload | Purpose |
|-------|---------|---------|
| `event.custom_scheduled_start` | `device` (`"dishwasher"`), `mode` (`auto`, `eco`, etc.), `max_delay_hours` | Fired from HA scripts/buttons to request a new optimized cycle. |

### Reminder Service
| Event | Payload Highlights | Notes |
|-------|--------------------|-------|
| `event.home_go_reminder_create` | `id`, `message`, optional `title`, `profile` (`normal`/`annoying`/`quiet`), `mode` (`repeating`/`single`), `start_time` or `initial_delay_minutes`, optional `speaker_entity`, `phone_notifier`, `visible_to`, quiet-hours config. | Creates or updates a reminder definition. Missing values fall back to sensible defaults. |
| `event.home_go_reminder_ack` | `id`, optional `user`. | Marks a reminder done. Single-time reminders are deleted immediately after ack. |
| `event.home_go_reminder_delete` | `id`. | Removes reminder definition/runtime/view state. |

### Notification Relay
| Event | Payload | Purpose |
|-------|---------|---------|
| `event.custom_notify` | `device`, `type`, `message`, optional data (speaker, push target, etc.). | Common event all components fire when they want HA to handle TTS/push delivery. |

---

## Dashboard Example – Mushroom Chips

The reminder component now writes a UI-friendly helper per user (e.g., `input_text.home_go_reminders_ui_alexey`). Its value is a semicolon-separated list like `rem-1|Take pills;rem-2|Stretch`, which keeps Lovelace simple. The following card turns those entries into interactive Mushroom chips:

```yaml
type: custom:config-template-card
entities:
  - input_text.home_go_reminders_ui_alexey
variables:
  reminders: >
    [[[ const raw = states['input_text.home_go_reminders_ui_alexey'].state;
        if (!raw || raw === 'unknown') return [];
        return raw.split(';').filter(Boolean).map((entry) => {
          const [id, label] = entry.split('|');
          return { id, label };
        });
    ]]]
card:
  type: custom:mushroom-chips-card
  chips: >
    [[[
      const chips = [];
      variables.reminders.forEach((rem) => {
        chips.push({
          type: "template",
          icon: rem.icon || "mdi:alarm",
          content: rem.label,
          multiline: true,
          tap_action: {
            action: "call-service",
            service: "event.fire",
            data: {
              event_type: "home_go_reminder_ack",
              event_data: { id: rem.id, user: "alexey" },
            },
          },
          hold_action: {
            action: "call-service",
            service: "event.fire",
            data: {
              event_type: "home_go_reminder_delete",
              event_data: { id: rem.id },
            },
          },
        });
      });
      if (!chips.length) {
        chips.push({
          type: "template",
          icon: "mdi:check-circle",
          content: "No reminders 🎉",
          tap_action: { action: "none" },
        });
      }
      return chips;
    ]]]
```

This creates one chip per reminder; tapping a chip acknowledges it, while holding deletes it. Duplicate the card for other household members by swapping the helper (e.g., `input_text.home_go_reminders_ui_pok`).

---

## Commands You’ll Actually Use
| Command | Description |
|---------|-------------|
| `make run` | Compile & run against `.env`. |
| `DRY_RUN=true make run` | Exercise logic without touching devices. |
| `make test` | Go test suite (includes reminder/jsonstore tests). |
| `make generate` | Refresh `entities/entities.go` from HA. |
| `make build` | Produce `bin/home-go`. |

---

Questions / tweaks? Update `AGENTS.md` for coding conventions and keep HA + Go changes in lockstep (submodule commit + parent repo commit).***
