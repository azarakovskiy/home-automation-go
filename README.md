# home-go

Event-driven home automation in Go with intelligent electricity price optimization.

Built on [gome-assistant](https://github.com/saml-dev/gome-assistant) library.

## Features

### Smart Charger Optimization
Intelligent electricity price-based charging for battery-powered devices:
- **Laptop Charger**: 6h charging optimized within 12h window
- **Vacuum Charger**: 1h charging optimized within 12h window
- 15-minute optimization cycles aligned with pricing granularity
- Automatic safety shutoff when away >2h
- Finds cheapest time slots for continuous charging

### Dishwasher Scheduler
Automatically schedules dishwasher to run during cheapest electricity periods:
- 5 operating modes with measured durations: Auto (137min), AutoQuick (70min), Eco (4h), Intensive (3h), Quick (1h)
- Price optimization with weighted stage importance based on actual power consumption
- Dynamic savings threshold using exponential decay
- Night mode: accepts any positive savings
- State persistence survives service restarts
- Human-readable TTS notifications via Home Assistant
- Custom event trigger from Home Assistant

### Price Optimization Engine
Unified optimizer supporting two strategies:
- **Scheduled Optimization**: Best start time for fixed-duration cycles (dishwasher, dryer)
- **Continuous Optimization**: Cheapest time slots within a window (chargers, heating)
- Weighted cost optimization (prioritize high-power stages)
- Dynamic savings thresholds based on available wait time
- Graceful degradation with insufficient data

### Architecture
- Component-based system with self-contained modules
- Type-safe event handling using Go generics
- 4 listener types: EventListener, EntityListener, DailySchedule, Interval
- Presence detection with house mode integration
- Event-based notifications (TTS via Home Assistant automations)
- Dry-run mode for safe testing

## Quick Start

### Prerequisites

- Go 1.23+
- Home Assistant instance
- Long-lived access token from Home Assistant

### Setup

1. **Clone and build**:
   ```bash
   git clone <your-repo>
   cd home-automation-go
   make build
   ```

2. **Configure environment**:
   ```bash
   cp env.example .env
   # Edit .env with your HA_URL and HA_AUTH_TOKEN
   # Optional: Set DRY_RUN=true for testing without device control
   ```

3. **Generate entities** (optional):
   ```bash
   cp entities/gen.yaml.example entities/gen.yaml
   # Edit entities/gen.yaml with your HA details
   make generate
   ```

4. **Run**:
   ```bash
   make run
   ```

## Docker

### Local Development
```bash
docker compose up -d
```

### Production (GHCR)
```bash
# Pull latest image
docker pull ghcr.io/<your-username>/home-go:main

# Run container
docker run -d --name home-go-automation \
  -e HA_URL=http://<ha-host>:8123 \
  -e HA_AUTH_TOKEN=<your-token> \
  --restart unless-stopped \
  ghcr.io/<your-username>/home-go:main
```

## Development

### Available Commands
- `make build` - Build binary
- `make run` - Build and run
- `make test` - Run tests with coverage
- `make lint` - Run linter
- `make fmt` - Format code
- `make generate` - Generate entities from HA
- `make mocks` - Generate test mocks
- `make install-mockgen` - Install mockgen tool
- `make tidy` - Clean up dependencies

### Dry-Run Mode
Test automation logic without controlling real devices:
```bash
DRY_RUN=true make run
```
All device control calls will be logged but not executed. Perfect for:
- Testing logic without affecting physical devices
- Development without Home Assistant connection
- Validating automation flows safely

### CI/CD
- **PRs**: Run linting and tests
- **Push to main**: Build and publish Docker image to GHCR
- **Tags**: Create tagged releases

### Test Coverage
```
pricing:    96.6%
optimizer:  90.5%
component:  43.7%
dishwasher: 13.8%
```

Run tests with coverage:
```bash
make test
```

## Configuration

### Environment Variables
- `HA_URL` - Home Assistant URL (default: http://192.168.1.43:8123)
- `HA_AUTH_TOKEN` - Long-lived access token
- `DRY_RUN` - Enable dry-run mode (logs actions without execution)
- `DEBUG` - Enable verbose debug logging

### Entity Generation
The `entities/` directory contains generated type-safe constants for your Home Assistant entities. Regenerate when you add new entities:

```bash
make generate
```

## Project Structure

```
├── component/          # Component framework
│   ├── component.go   # Component interface & Base with house mode helpers
│   └── event_handler.go # Type-safe event handling
├── pricing/           # Electricity pricing service
│   └── service.go
├── scheduler/
│   ├── optimizer/     # Unified optimization engine
│   │   └── optimizer.go  # Scheduled & continuous optimization
│   ├── profile.go     # Generic profile for scheduled devices
│   └── dishwasher/    # Dishwasher implementation
│       ├── component.go
│       ├── profile.go # Device-specific profiles
│       └── types.go
├── charger/
│   ├── profiles.go    # Charging profiles (Laptop, Vacuum)
│   ├── laptop/        # Laptop charger component
│   └── vacuum/        # Vacuum charger component
├── notifications/     # Event-based notification service
│   └── service.go
├── debug/             # Debug logging utility
├── dryrun/            # Dry-run mode wrapper
├── entities/          # Generated HA entities
│   ├── entities.go    # Generated constants
│   └── custom_entities.go # Custom events
├── mocks/             # Generated test mocks
├── main.go            # Application entry
├── AGENTS.md          # AI agent instructions
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

## License

MIT
