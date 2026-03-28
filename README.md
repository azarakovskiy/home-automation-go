# home-go

Event-driven home automation in Go for pricing-aware scheduling and charging.

## Requirements

- Go 1.23+
- Home Assistant instance
- Long-lived Home Assistant access token

## Local Development

```bash
make build
make run
make test
make lint
make fmt
make mocks
make tidy
```

Dry-run mode logs device actions instead of executing them:

```bash
DRY_RUN=true make run
```

## Environment Variables

- `HA_URL` - Home Assistant URL
- `HA_AUTH_TOKEN` - Long-lived access token
- `DRY_RUN` - Enable dry-run mode
- `DEBUG` - Enable verbose debug logging

## Entity Generation

`make generate` runs the Home Assistant entity generator and requires access to a real Home Assistant instance. It is a local/manual step and is not verified in CI.

Use [`gen.yaml.example`](/Users/alexey/dev/azarakovskiy/home-automation-go/gen.yaml.example) as the starting point for generator configuration.

## Runtime MQTT Entities

This repo also supports runtime-created Home Assistant entities through the MQTT-based runtime in [`internal/tech/homeassistant/entities/`](/Users/alexey/dev/azarakovskiy/home-automation-go/internal/tech/homeassistant/entities).

Principles:

- Use generated `entities.*` constants for stable Home Assistant entities that already exist in the installation.
- Use Home Assistant helpers for persisted user-managed state such as schedules, toggles, and settings.
- Use the runtime MQTT entity component when the Go service is the owner of short-lived or service-defined entities that should appear in Home Assistant dynamically.
- Runtime entity declaration is init-safe: it publishes discovery metadata, but does not overwrite an existing retained state value.
- Runtime state changes are explicit via typed setters such as switch on/off or sensor value updates.
- Cleanup is explicit through runtime removal/reconcile, with a small local registry file used only to remember which runtime entities this service owns across restarts.

## Repository Layout

- [`cmd/home-go/`](/Users/alexey/dev/azarakovskiy/home-automation-go/cmd/home-go) - application entrypoint
- [`internal/config/`](/Users/alexey/dev/azarakovskiy/home-automation-go/internal/config) - environment-backed runtime configuration
- [`internal/domain/`](/Users/alexey/dev/azarakovskiy/home-automation-go/internal/domain) - pure scheduling, charging, pricing, and optimization logic
- [`internal/tech/homeassistant/`](/Users/alexey/dev/azarakovskiy/home-automation-go/internal/tech/homeassistant) - Home Assistant adapters, entities, and device orchestration
- [`internal/tech/runtime/`](/Users/alexey/dev/azarakovskiy/home-automation-go/internal/tech/runtime) - runtime wrappers such as debug and dry-run helpers
- [`internal/mocks/`](/Users/alexey/dev/azarakovskiy/home-automation-go/internal/mocks) - generated mocks

## CI and Images

- Pull requests run lint, tests, mock drift checks, and publish preview images tagged `pr-<number>` and `pr-<number>-<sha>`
- Pushes to `master` publish `ghcr.io/<owner>/home-go:master` and `ghcr.io/<owner>/home-go:sha-<commit>`
- Tags matching `v*` publish versioned images and `latest`

## Deployment

- The VM compose directory is expected to read `IMAGE_TAG` from a local `.env` file and default to `master`
- A VM-local cron job runs [`scripts/deploy.sh`](/Users/alexey/dev/azarakovskiy/home-automation-go/scripts/deploy.sh) every minute and reconciles the running container to `IMAGE_TAG`
- The VM authenticates to GHCR once with a personal access token with `read:packages`, then reuses Docker's stored credentials for polling
- `master` auto-deploys within one minute because the VM normally tracks `IMAGE_TAG=master`
- To test a pull request image manually, set `IMAGE_TAG=pr-<number>` on the VM and run the deploy script once or wait for the next cron run
- To roll back, set `IMAGE_TAG=master` or a specific `sha-<commit>` on the VM
- Closed pull requests trigger GHCR cleanup for their `pr-<number>` image versions

## License

MIT
