# Hosthalla

Self-hosted infrastructure dashboard for managing hosts, SSH credentials, and monitoring agents.

- **Host inventory** — add hosts by IP, group them with tags, store SSH access methods
- **ICMP monitoring** — ping individual hosts or all at once directly from the UI
- **Remote agents** — install a lightweight agent on any Linux machine; it streams system info and live metrics (CPU, memory, disk, network) back to the dashboard
- **API tokens** — issue scoped tokens for agent registration and API access

## Tech Stack

| Layer | Technology |
|---|---|
| Language | Go 1.26 |
| HTTP | `net/http` (Go 1.22+ routing) |
| Database | PostgreSQL 18 |
| UI | [Templ](https://templ.guide) + [HTMX](https://htmx.org) |
| Auth | Cookie sessions + bcrypt |
| API auth | `hht_`-prefixed tokens (SHA-256 stored) |
| Agent metrics | `gopsutil/v4` |

## Requirements

- Go 1.26+
- Docker + Docker Compose (for local PostgreSQL)

## Development Setup

### 1. Start the database

```bash
make dev-up
```

This starts a PostgreSQL 18 container and waits until it is healthy.
Connection string: `postgres://hosthalla:hosthalla@localhost:5432/hosthalla`

### 2. Apply migrations

```bash
make migrate-up
```

### 3. Generate the config file

```bash
go run ./cmd/cli config generate
```

The default config is written to `~/.hosthalla/config.yaml`.
Use `--path` to write it elsewhere.

### 4. Edit the config

```yaml
web:
  host: 0.0.0.0
  port: 8080
database:
  host: localhost
  port: 5432
  user: hosthalla
  password: hosthalla
  database: hosthalla
log_level: warning   # debug | info | warning | error
```

### 5. Create the first user

```bash
go run ./cmd/cli create-user <username> <password>
```

Password must be at least 8 characters.

### 6. Start the web server

```bash
make dev-web          # regenerates Templ files, then starts the server
# or
go run ./cmd/web
```

The UI is available at `http://localhost:8080`.

## Building

```bash
make build-web
```

Produces `dist/hosthalla` with version, commit, and build timestamp embedded via ldflags.

## CLI Reference

The CLI binary (`cmd/cli`) handles everything except serving the UI.

### Config commands

```bash
# Generate default config at ~/.hosthalla/config.yaml
go run ./cmd/cli config generate [--path <file>] [--overwrite]

# Print the current config
go run ./cmd/cli config show [--path <file>]
```

### User management

```bash
go run ./cmd/cli create-user <username> <password>
```

### Agent commands

```bash
# Register this machine as an agent for a host
# (The recommended way is to use the "Register Agent" button in the UI,
#  which generates the full command with a pre-filled token.)
go run ./cmd/cli agent register \
  --host <server-url> \
  --host-id <uuid> \
  --token <hht_...>

# Start the agent worker (heartbeat + metrics loop)
go run ./cmd/cli agent run [--config <file>]
```

Agent config is saved to `~/.hosthalla/agent.yaml` by default.
The agent sends a heartbeat every **5 seconds** and metrics every **30 seconds**.

## Monitoring Agents

1. Open the dashboard and navigate to a host.
2. Click **Register Agent** — a shell command with a scoped API token is generated.
3. Run that command on the target machine. It registers the agent and uploads system info.
4. Run `hosthalla agent run` on the target machine (or set it up as a systemd service).

The dashboard then shows live CPU, memory, disk, and network metrics for the host.

## Make Targets

| Target | Description |
|---|---|
| `make dev-up` | Start PostgreSQL container |
| `make dev-down` | Stop PostgreSQL container |
| `make dev-reset` | Stop and remove container + volumes |
| `make dev-logs` | Follow container logs |
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Roll back the last migration |
| `make templ-generate` | Regenerate `*_templ.go` files |
| `make build-web` | Build production binary to `dist/hosthalla` |
| `make dev-web` | Regenerate Templ files + run the web server |

## Project Structure

```
cmd/
  cli/          # CLI entry point (config, users, agent)
  web/          # Web server entry point
internal/
  agent/        # Agent model, config, gopsutil metrics, worker loop
  api/          # REST API for agents (/api/v1/...)
  authentication/ # Sessions, API tokens, bcrypt passwords
  cli/          # CLI command implementations
  config/       # App config struct, load/save
  host/         # Host domain: model, service, repository interfaces
  logger/       # slog setup
  version/      # Version string injected via ldflags
  web/          # Server-rendered UI handlers and middleware
migrations/     # SQL migration pairs (up/down)
ui/             # Templ components (Feature-Sliced Design)
  app/layout/
  entities/
  features/
  pages/
  shared/ui/
  widgets/
infra/dev/      # docker-compose.yaml for local PostgreSQL
```

## License

MIT — see [LICENSE](LICENSE).
