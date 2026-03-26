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

## Repository Layout

- [`component/`](/Users/alexey/dev/azarakovskiy/home-automation-go/component) - shared component helpers and state persistence
- [`pricing/`](/Users/alexey/dev/azarakovskiy/home-automation-go/pricing) - electricity price cache and classification
- [`optimization/optimizer/`](/Users/alexey/dev/azarakovskiy/home-automation-go/optimization/optimizer) - shared optimization engine
- [`optimization/scheduled/`](/Users/alexey/dev/azarakovskiy/home-automation-go/optimization/scheduled) - scheduled devices such as the dishwasher
- [`optimization/continuous/`](/Users/alexey/dev/azarakovskiy/home-automation-go/optimization/continuous) - continuous charging devices such as laptop and vacuum chargers
- [`notifications/`](/Users/alexey/dev/azarakovskiy/home-automation-go/notifications) - custom event notifications for Home Assistant
- [`entities/`](/Users/alexey/dev/azarakovskiy/home-automation-go/entities) - generated and custom entity constants
- [`dryrun/`](/Users/alexey/dev/azarakovskiy/home-automation-go/dryrun) - dry-run wrapper for device actions

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
