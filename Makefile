COMPOSE_FILE := infra/dev/docker-compose.yaml
COMPOSE_PROJECT := $(notdir $(patsubst %/,%,$(dir $(COMPOSE_FILE))))
COMPOSE_NETWORK := $(COMPOSE_PROJECT)_default
COMPOSE := docker compose -p $(COMPOSE_PROJECT) -f $(COMPOSE_FILE)

DATABASE_URL := postgres://hosthalla:hosthalla@localhost:5432/hosthalla?sslmode=disable
DATABASE_URL_DOCKER := postgres://hosthalla:hosthalla@postgres:5432/hosthalla?sslmode=disable
MIGRATE_IMAGE := migrate/migrate:v4.18.2
DIST_DIR := dist
HOSTHALLA_BINARY := $(DIST_DIR)/hosthalla

VERSION_VERSION := $(shell git describe --tags --always --dirty)
VERSION_COMMIT := $(shell git rev-parse --short HEAD)
VERSION_BUILD_AT := $(shell date -u +%FT%TZ)
VERSION_LDFLAGS := \
	-X github.com/yazmeyaa/hosthalla/internal/version.Version=$(VERSION_VERSION) \
	-X github.com/yazmeyaa/hosthalla/internal/version.Commit=$(VERSION_COMMIT) \
	-X github.com/yazmeyaa/hosthalla/internal/version.BuildAt=$(VERSION_BUILD_AT)

LDFLAGS := -ldflags "$(VERSION_LDFLAGS)"
LDFLAGS_BUILD := -ldflags "-s -w $(VERSION_LDFLAGS)"

.DEFAULT_GOAL := help

.PHONY: help dev dev-up dev-down dev-logs dev-ps dev-reset migrate-up migrate-down templ-generate build build-hosthalla dev-run-web dev-web

# Start dev infrastructure and wait until services are healthy.
dev: dev-up ## Start development infrastructure.

help: ## Show available make targets.
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*## "}; /^[a-zA-Z0-9][^:]*:.*## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

dev-up: ## Start dev infrastructure and wait for health checks.
	$(COMPOSE) up -d --wait
	@echo "Dev environment is ready."
	@echo "PostgreSQL: $(DATABASE_URL)"

dev-down: ## Stop dev infrastructure.
	$(COMPOSE) down

dev-logs: ## Stream infrastructure logs.
	$(COMPOSE) logs -f

dev-ps: ## Show infrastructure service status.
	$(COMPOSE) ps

# Stop services and remove persisted volumes.
dev-reset: ## Stop infra and remove persisted volumes.
	$(COMPOSE) down -v

migrate-up: dev-up ## Apply all pending migrations in docker network.
	docker run --rm \
		-v "$(CURDIR)/migrations:/migrations" \
		--network $(COMPOSE_NETWORK) \
		$(MIGRATE_IMAGE) \
		-path=/migrations \
		-database "$(DATABASE_URL_DOCKER)" \
		up

migrate-down: dev-up ## Roll back one migration in docker network.
	docker run --rm \
		-v "$(CURDIR)/migrations:/migrations" \
		--network $(COMPOSE_NETWORK) \
		$(MIGRATE_IMAGE) \
		-path=/migrations \
		-database "$(DATABASE_URL_DOCKER)" \
		down 1

templ-generate: ## Regenerate templ views.
	go tool templ generate

build: build-hosthalla ## Build binary locally.

build-hosthalla: templ-generate ## Build unified Hosthalla binary from cmd/hosthalla.
	mkdir -p $(DIST_DIR)
	go build \
	$(LDFLAGS_BUILD) \
		-o $(HOSTHALLA_BINARY) \
		./cmd/hosthalla

dev-run-web: ## Run web server locally from source.
	go run \
	$(LDFLAGS) \
		./cmd/hosthalla serve


dev-web: templ-generate dev-run-web ## Regenerate templates and run web server.
