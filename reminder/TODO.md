# Reminder TODO

## Go service
- [ ] Extract reusable JSON store helpers into a shared package if other components adopt the pattern.
- [ ] Add richer validation/logging around malformed reminder events before enabling more profiles.

## Home Assistant / Dashboard
- [ ] Declare `input_text.reminders_config`, `input_text.reminders_runtime`, and `input_text.reminders_views` plus the three `event.custom_reminder_*` entries in HA YAML, then regenerate `entities`.
- [ ] Update Lovelace buttons/scripts to fire `event.custom_reminder_create/ack/delete` payloads and render reminder views per user.
