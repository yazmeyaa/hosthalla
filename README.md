# Hosthalla

Hosthalla is a Go web application with PostgreSQL-backed storage and server-side UI templates.

## Requirements

- Go 1.26+
- Docker + Docker Compose

## Development Setup

1. Start local infrastructure:

```bash
make dev-up
```

2. Apply database migrations:

```bash
make migrate-up
```

3. Create the app config (or provide your own path with `-config`):

```bash
go run ./cmd/web
```

The default config file path is `~/.hosthalla/config.yaml`. If the file does not exist, Hosthalla creates a template and exits so you can fill it in.

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
```

4. Run the web server:

```bash
go run ./cmd/web
```

## Useful Commands

- Regenerate templ files: `make templ-generate`
- Create a user: `go run ./cmd/cli create-user <username> <password>`
- Stop dev services: `make dev-down`
- Reset dev services and volumes: `make dev-reset`

## License

Licensed under the MIT License. See `LICENSE`.
