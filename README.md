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

- PostgreSQL 18+ (for running the app)
- Go 1.26+ (only if you build from source)

## Install Script

```sh
#!/usr/bin/env bash
set -euo pipefail

REPO="yazmeyaa/hosthalla"
ARCHIVE_PATTERN="linux_amd64"

URL=$(curl -s https://api.github.com/repos/$REPO/releases/latest \
  | jq -r --arg pattern "$ARCHIVE_PATTERN" '.assets[] | select(.name | test($pattern)) | .browser_download_url' \
  | head -n 1)

if [ -z "$URL" ] || [ "$URL" = "null" ]; then
  echo "Could not find release asset for pattern: $ARCHIVE_PATTERN" >&2
  exit 1
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -L -o "$TMP/pkg.tar.gz" "$URL"
tar -xzf "$TMP/pkg.tar.gz" -C "$TMP"

for bin in hosthalla; do
  if [ -f "$TMP/$bin" ]; then
    sudo install -m 0755 "$TMP/$bin" "/usr/local/bin/$bin"
  fi
done
```

## Quick Start

### 1. Install binary

Run the install script above, or download the latest release asset and place `hosthalla` in your `PATH`.

### 2. Generate app config

```sh
hosthalla config generate
```

Default path: `~/.hosthalla/config.yaml`.

### 3. Fill the config

```yml
web:
  host: 0.0.0.0
  port: 8080
database:
  host: <postgres-host>
  port: 5432
  user: <postgres-user>
  password: <postgres-password>
  database: <postgres-database>
log_level: warning   # debug | info | warning | error
```

### 4. Apply migrations

```sh
hosthalla db migrate
```

### 5. Start Hosthalla

```sh
hosthalla serve
```

The UI is available at `http://localhost:8080`.

### 6. (Optional) Create first user from CLI

```sh
hosthalla users create <username> <password>
```

## Building

```bash
make build
```

Builds the binary locally:
- `dist/hosthalla`

Release binaries include version, commit, and build timestamp via ldflags.

## CLI Reference

The `hosthalla` binary exposes the server, local agent, and administration
commands through one explicit command tree. Legacy command aliases are not
supported.

### Help

```sh
hosthalla help
# or
hosthalla --help
```

### Config commands

```sh
# Generate default config at ~/.hosthalla/config.yaml
hosthalla config generate [--path <file>] [--overwrite]

# Print the current config
hosthalla config show [--path <file>]

# Validate the current config
hosthalla config validate [--path <file>]
```

### User management

```sh
hosthalla users create <username> <password>
```

### Database commands

```sh
# Apply all pending migrations
hosthalla db migrate
```

### Agent commands

```sh
# Register this machine as an agent for a host
# (The recommended way is to use the "Register Agent" button in the UI,
#  which generates the full command with a pre-filled token.)
hosthalla agent register \
  --host <server-url> \
  --host-id <uuid> \
  --token <hht_...>

# Start the agent worker (heartbeat + metrics loop)
hosthalla agent run [--config <file>]
```

Agent config is saved to `~/.hosthalla/agent.yaml` by default.
The agent sends a heartbeat every **5 seconds** and metrics every **30 seconds**.

## Monitoring Agents

1. Open the dashboard and navigate to a host.
2. Click **Register Agent** — a shell command with a scoped API token is generated.
3. Run `hosthalla agent register ...` on the target machine.
4. Run `hosthalla agent run` on the target machine (or set it up as a systemd service).

The dashboard then shows live CPU, memory, disk, and network metrics for the host.

## Agent Quick Start

```sh
# 1) Register agent on target host
hosthalla agent register --host <server-url> --host-id <uuid> --token <hht_...>

# 2) Start agent loop
hosthalla agent run
```

## Make Targets

| Target | Description |
|---|---|
| `make migrate-up` | Apply all pending migrations |
| `make migrate-down` | Roll back the last migration |
| `make templ-generate` | Regenerate `*_templ.go` files |
| `make help` | Show available Make targets |
| `make build` | Build Hosthalla binary |
| `make build-hosthalla` | Build binary to `dist/hosthalla` |
| `make dev-web` | Regenerate Templ files + run the web server |

## Project Structure

```
cmd/
  hosthalla/    # Unified CLI entry point
internal/
  agent/        # Agent model, config, gopsutil metrics, worker loop
  api/          # REST API for agents (/api/v1/...)
  authentication/ # Sessions, API tokens, bcrypt passwords
  cli/          # CLI command tree runner
  commands/     # Hosthalla command implementations
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
infra/dev/      # local development infrastructure files
```

## License

MIT — see [LICENSE](LICENSE).
