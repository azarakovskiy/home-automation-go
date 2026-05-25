# Price Announcer — Home Assistant Integration

The price announcer delivers two kinds of messages: on-demand price summaries (triggered by an automation) and reactive extreme-price alerts (triggered automatically when prices update). Both are delivered as HA custom events.

---

## On-demand summary

### What it does

Fires a time-aware natural-language summary of electricity prices. Before 11:00 it covers the full day ahead; from 11:00 it covers the remaining window until midnight.

### HA entity

The app registers an MQTT switch via auto-discovery:

| Property | Value |
|---|---|
| Entity ID | `switch.price_summary_trigger` |
| Name | Price Summary |
| Icon | `mdi:currency-eur` |

The switch is a momentary trigger: the app fires the summary when it receives an ON command but does **not** reset it back to OFF — your automation must do that.

### Example automation

```yaml
alias: Morning price briefing
trigger:
  - platform: time
    at: "08:00:00"
action:
  - service: switch.turn_on
    target:
      entity_id: switch.price_summary_trigger
  - delay: "00:00:02"
  - service: switch.turn_off
    target:
      entity_id: switch.price_summary_trigger
```

---

## Reactive extreme-price alerts

No HA-side trigger setup is needed. The app watches `sensor.frank_energie_prices_current_electricity_price_all_in` and fires an alert automatically when it detects a qualifying run of extreme prices (spike × median for a minimum duration). Alerts are deduplicated in memory — one alert per run.

The alert is suppressed when the house is in Night or Away mode.

---

## Receiving notifications

Both summaries and alerts are fired as `custom_notify` events on the HA event bus. Listen for them in an automation and forward to a notification target of your choice.

### Event structure

| Field | Description |
|---|---|
| `device` | Always `"pricing"` |
| `type` | `"price_day_summary"` or `"price_extreme_alert"` |
| `message` | Human-readable text ready to send as-is |
| `kind` | *(extreme alerts only)* `"extreme prices"` or `"negative prices"` |
| `from` | *(extreme alerts only)* RFC3339 start of the extreme run |
| `till` | *(extreme alerts only)* RFC3339 end of the extreme run |
| `duration_h` | *(extreme alerts only)* run length in whole hours |

### Example automation

```yaml
alias: Price announcer → mobile notification
trigger:
  - platform: event
    event_type: custom_notify
    event_data:
      device: pricing
action:
  - service: notify.mobile_app_your_phone
    data:
      message: "{{ trigger.event.data.message }}"
      title: >
        {% if trigger.event.data.type == 'price_extreme_alert' %}
          ⚡ Price alert
        {% else %}
          💡 Price summary
        {% endif %}
```

Replace `notify.mobile_app_your_phone` with your actual notification target.
