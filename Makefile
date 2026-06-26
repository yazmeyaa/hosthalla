COMPOSE_FILE := infra/dev/docker-compose.yaml
COMPOSE_PROJECT := $(notdir $(patsubst %/,%,$(dir $(COMPOSE_FILE))))
COMPOSE_NETWORK := $(COMPOSE_PROJECT)_default
COMPOSE := docker compose -p $(COMPOSE_PROJECT) -f $(COMPOSE_FILE)

DATABASE_URL := postgres://hosthalla:hosthalla@localhost:5432/hosthalla?sslmode=disable
DATABASE_URL_DOCKER := postgres://hosthalla:hosthalla@postgres:5432/hosthalla?sslmode=disable
MIGRATE_IMAGE := migrate/migrate:v4.18.2

VERSION_VERSION := $(shell git describe --tags --always --dirty)
VERSION_COMMIT := $(shell git rev-parse --short HEAD)
VERSION_BUILD_AT := $(shell date -u +%FT%TZ)
VERSION_LDFLAGS := \
	-X github.com/yazmeyaa/hosthalla/internal/version.Version=$(VERSION_VERSION) \
	-X github.com/yazmeyaa/hosthalla/internal/version.Commit=$(VERSION_COMMIT) \
	-X github.com/yazmeyaa/hosthalla/internal/version.BuildAt=$(VERSION_BUILD_AT)

LDFLAGS := -ldflags "$(VERSION_LDFLAGS)"
LDFLAGS_BUILD := -ldflags "-s -w $(VERSION_LDFLAGS)"

.PHONY: dev dev-up dev-down dev-logs dev-ps dev-reset migrate-up migrate-down templ-generate build-web dev-run-web dev-web

# Start dev infrastructure and wait until services are healthy.
dev: dev-up

dev-up:
	$(COMPOSE) up -d --wait
	@echo "Dev environment is ready."
	@echo "PostgreSQL: $(DATABASE_URL)"

dev-down:
	$(COMPOSE) down

dev-logs:
	$(COMPOSE) logs -f

dev-ps:
	$(COMPOSE) ps

# Stop services and remove persisted volumes.
dev-reset:
	$(COMPOSE) down -v

migrate-up: dev-up
	docker run --rm \
		-v "$(CURDIR)/migrations:/migrations" \
		--network $(COMPOSE_NETWORK) \
		$(MIGRATE_IMAGE) \
		-path=/migrations \
		-database "$(DATABASE_URL_DOCKER)" \
		up

migrate-down: dev-up
	docker run --rm \
		-v "$(CURDIR)/migrations:/migrations" \
		--network $(COMPOSE_NETWORK) \
		$(MIGRATE_IMAGE) \
		-path=/migrations \
		-database "$(DATABASE_URL_DOCKER)" \
		down 1

templ-generate:
	go tool templ generate

build-web:
	go build \
	$(LDFLAGS_BUILD) \
	-o dist/web \
	cmd/web/web.go

dev-run-web:
	go run \
	$(LDFLAGS) \
		cmd/web/web.go


dev-web: templ-generate dev-run-web