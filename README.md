# home-go

Event-driven home automation in Go with intelligent electricity price optimization.

Built on [gome-assistant](https://github.com/saml-dev/gome-assistant) library.

## Features

### Dishwasher Scheduler
Automatically schedules dishwasher to run during cheapest electricity periods:
- 5 operating modes: Eco (4h), Auto (3h), AutoQuick (2h), Intensive (3h), Quick (1h)
- Price optimization with weighted stage importance
- State persistence survives service restarts
- Custom event trigger from Home Assistant

### Price Optimization Engine
Generic optimizer for any cyclic appliance:
- Calculates cheapest time window within deadline
- Weighted cost optimization (prioritize high-power stages)
- Percentage-based savings thresholds
- Graceful degradation with insufficient data

### Architecture
- Component-based system with self-contained modules
- Type-safe event handling using Go generics
- 4 listener types: EventListener, EntityListener, DailySchedule, Interval
- Generic state manager for device persistence

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

### Entity Generation
The `entities/` directory contains generated type-safe constants for your Home Assistant entities. Regenerate when you add new entities:

```bash
make generate
```

## Project Structure

```
├── component/          # Component framework
│   ├── component.go   # Component interface & Base
│   ├── event_handler.go # Type-safe event handling
│   └── state.go       # Generic state persistence
├── pricing/           # Electricity pricing service
│   └── service.go
├── scheduler/
│   ├── optimizer/     # Generic optimization engine
│   │   ├── optimizer.go
│   │   └── profile.go # Weight constants
│   ├── dishwasher/    # Dishwasher implementation
│   │   ├── component.go
│   │   ├── controller.go
│   │   ├── state_manager.go
│   │   ├── profile.go
│   │   └── types.go
│   └── types.go       # Shared types
├── entities/          # Generated HA entities
│   ├── entities.go    # Generated constants
│   ├── events.go      # Event wrappers
│   └── custom_entities.go
├── mocks/             # Generated test mocks
├── main.go            # Application entry
├── Dockerfile
├── docker-compose.yml
└── Makefile
```

## License

MIT
