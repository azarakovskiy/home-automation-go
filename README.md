# home-go

Home Assistant automation written in Go using the [gome-assistant](https://github.com/saml-dev/gome-assistant) library.

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
- `make test` - Run tests
- `make lint` - Run linter
- `make fmt` - Format code
- `make generate` - Generate entities from HA
- `make tidy` - Clean up dependencies

### CI/CD
- **PRs**: Run linting and tests
- **Push to main**: Build and publish Docker image to GHCR
- **Tags**: Create tagged releases

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
├── entities/           # Generated HA entity constants
├── .github/workflows/  # CI/CD configuration
├── main.go            # Application entrypoint
├── Dockerfile         # Container image
├── docker-compose.yml # Local development
└── Makefile          # Build commands
```

## License

MIT
