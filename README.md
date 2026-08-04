# Hosthalla

A self-hosted dashboard for tracking servers, storing access methods, and keeping a lightweight eye on your infrastructure.

<p align="center">
  <img src="./docs/hosthalla_dashboard_1.jpg" alt="Hosthalla dashboard" width="900">
</p>

Hosthalla brings hosts, tags, SSH access methods, availability checks, and live machine metrics into one small web interface. It is built for self-hosted setups, homelabs, personal servers, and small teams that need a practical internal inventory without running a full CMDB.

## Features

- **Host inventory**: store name, description, IP address, tags, and filter hosts by tags.
- **Management methods**: store SSH password and SSH key access methods, with secrets encrypted by `security.secret_encryption_key`.
- **Availability checks**: run ICMP ping for a single host or all hosts from the web UI.
- **Monitoring agents**: run a lightweight local agent that sends heartbeat, system information, and CPU, memory, disk, and network metrics.
- **Live dashboard**: view infrastructure overview, agent status, latest metrics, and live updates through WebSocket/HTMX.
- **Import and export**: move hosts and management methods through JSON.
- **Users and API tokens**: cookie sessions for the UI, `hht_` API tokens for API access and agent registration.
- **Unified CLI**: server, migrations, bootstrap, users, tokens, hosts, and agents are managed through one `hosthalla` binary.

## Why Use It

Hosthalla answers a practical day-to-day question: what machines do I have, how do I access them, are they reachable, and what resources are they using right now? It is not a replacement for Prometheus, Ansible, or a full asset-management platform. It is a compact operational entry point for a small server fleet.

## Stack

| Layer | Technology |
| --- | --- |
| Backend | Go 1.26, `net/http` |
| Database | PostgreSQL 18 |
| UI | Templ, HTMX, WebSocket |
| Auth | Cookie sessions, bcrypt |
| API auth | Bearer tokens with the `hht_` prefix, SHA-256 hash stored in DB |
| Agent metrics | `gopsutil/v4` |
| Dev infra | Docker Compose |

## Development Quick Start

Requirements: Go 1.26+, Docker, and Docker Compose.

```sh
make infra-up
go run ./cmd/hosthalla config generate
go run ./cmd/hosthalla bootstrap --username admin --password admin
go run ./cmd/hosthalla serve
```

The web UI will be available at:

```text
http://localhost:8080
```

For regular local development, use:

```sh
make dev
```

`make dev` starts PostgreSQL, regenerates Templ views, and starts the server with `go run ./cmd/hosthalla serve`. Before the first run, create the config and run `bootstrap` once.

## Install Binary

Use the install script from the repository:

```sh
curl -fsSL https://raw.githubusercontent.com/yazmeyaa/hosthalla/main/scripts/install_hosthalla.sh | bash
```

The script requires `curl`, `jq`, `tar`, and `sudo`. You can also download a release asset manually and place the `hosthalla` binary somewhere in your `PATH`.

## Configuration

The default application config path is `~/.hosthalla/config.yaml`.

Generate a config:

```sh
hosthalla config generate
```

Example config:

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
security:
  secret_encryption_key: <base64-encoded-32-byte-key>
log_level: warning
```

`secret_encryption_key` is used to encrypt host management secrets. It is generated automatically when the config is created.

Validate the config:

```sh
hosthalla config validate
```

## First Run

Once PostgreSQL is available and the config is filled in, run bootstrap:

```sh
hosthalla bootstrap --username admin --password <strong-password>
```

This applies migrations and creates the first user. Then start the server:

```sh
hosthalla serve
```

Manual setup is also supported:

```sh
hosthalla db migrate
hosthalla users create admin <strong-password>
hosthalla serve
```

## Monitoring Agent

The agent runs on the machine you want to monitor. It registers with Hosthalla, stores its local config in `~/.hosthalla/agent.yaml`, then periodically sends heartbeat and metrics.

Recommended flow:

1. Open Hosthalla in the browser.
2. Create a host or open an existing one.
3. Click **Register Agent**.
4. Run the generated command on the target machine.
5. Start the agent:

```sh
hosthalla agent run
```

Manual registration looks like this:

```sh
hosthalla agent register \
  --host https://hosthalla.example.com \
  --host-id <host-uuid> \
  --token <hht_...>

hosthalla agent run
```

Current agent defaults: heartbeat every `2s`, metrics every `4s`. These intervals are saved in the agent config and can be updated through the server-side agent configuration.

## CLI

General form:

```sh
hosthalla [--config <file>] [--json] <command> [arguments]
```

Common commands:

```sh
hosthalla help
hosthalla version

hosthalla config generate [--path <file>] [--overwrite]
hosthalla config show [--path <file>]
hosthalla config validate [--path <file>]

hosthalla bootstrap [--username <username> --password <password>]

hosthalla db migrate
hosthalla db status [--json]
hosthalla db rollback

hosthalla users create <username> <password>
hosthalla users list [--json]
hosthalla users show <user-id-or-username> [--json]
hosthalla users password set <user-id-or-username> <password>
hosthalla users delete <user-id-or-username>

hosthalla tokens create --user <user-id-or-username> --name <name> [--scope <scope>] [--ttl <duration>] [--json]
hosthalla tokens list [--user <user-id-or-username>] [--json]
hosthalla tokens show <token-id> [--json]
hosthalla tokens revoke <token-id>

hosthalla hosts list [--json]
hosthalla hosts show <host-id> [--json]
hosthalla hosts delete <host-id>

hosthalla agents list [--json]
hosthalla agents show <agent-id> [--json]
hosthalla agents delete <agent-id>

hosthalla agent register --host <server-url> --host-id <uuid> --token <hht_...>
hosthalla agent run [--config <file>]
```

`tokens create` prints the plain token only once. Store it immediately.

## Make Targets

| Target | Description |
| --- | --- |
| `make help` | Show available Make targets |
| `make dev` | Start PostgreSQL, regenerate Templ files, and run the web server |
| `make run` | Run the web server from source |
| `make build` | Build the binary |
| `make generate` | Regenerate Templ Go files |
| `make test` | Run Go tests |
| `make check` | Regenerate Templ files and run Go tests |
| `make infra-up` | Start PostgreSQL for development |
| `make infra-down` | Stop development infrastructure |
| `make infra-status` | Show development service status |
| `make infra-logs` | Stream development infrastructure logs |
| `make infra-reset` | Stop development infrastructure and remove volumes |
| `make db-migrate` | Apply migrations in the Docker network |
| `make db-rollback` | Roll back one migration in the Docker network |

## Project Structure

```text
cmd/hosthalla/          # unified CLI entry point
internal/agent/         # agent, API client, system info, and metrics collection
internal/api/           # agent API: registration, heartbeat, metrics, config
internal/authentication/# users, sessions, API tokens
internal/commands/      # CLI command implementations
internal/config/        # config.yaml loading, generation, and validation
internal/host/          # host, metrics, and management method domain model
internal/web/           # web router, handlers, middleware
migrations/             # SQL migrations up/down
ui/                     # Templ UI using Feature-Sliced Design
infra/dev/              # Docker Compose files for local development
scripts/                # helper installation scripts
docs/                   # images and documentation
```

## Security

- User passwords are hashed with bcrypt.
- API tokens are stored as SHA-256 hashes.
- SSH management secrets are encrypted with `security.secret_encryption_key`.
- Host exports may include decrypted secrets when they can be read successfully. Treat exported JSON files as sensitive data.

## Build

```sh
make build
```

The binary is written to:

```text
dist/hosthalla
```

Release builds receive version, commit, and build timestamp through `ldflags`.

## License

MIT. See [LICENSE](LICENSE).
