COMPOSE_FILE := infra/dev/docker-compose.yaml
COMPOSE_PROJECT ?= hosthalla-dev
COMPOSE_NETWORK := $(COMPOSE_PROJECT)_default
COMPOSE := docker compose --project-name $(COMPOSE_PROJECT) --file $(COMPOSE_FILE)

DATABASE_URL_LOCAL := postgres://hosthalla:hosthalla@localhost:5432/hosthalla?sslmode=disable
DATABASE_URL_DOCKER := postgres://hosthalla:hosthalla@postgres:5432/hosthalla?sslmode=disable

APP := ./cmd/hosthalla
DIST_DIR := dist
HOSTHALLA_BIN := $(DIST_DIR)/hosthalla
MIGRATE_IMAGE := migrate/migrate:v4.18.2

GO ?= go
VERSION := $(shell git describe --tags --always --dirty)
COMMIT := $(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u +%FT%TZ)
VERSION_FLAGS := \
	-X github.com/yazmeyaa/hosthalla/internal/version.Version=$(VERSION) \
	-X github.com/yazmeyaa/hosthalla/internal/version.Commit=$(COMMIT) \
	-X github.com/yazmeyaa/hosthalla/internal/version.BuildAt=$(BUILD_TIME)

GO_RUN := $(GO) run -ldflags "$(VERSION_FLAGS)" $(APP)
GO_BUILD := $(GO) build -ldflags "-s -w $(VERSION_FLAGS)"
MIGRATE := docker run --rm \
	-v "$(CURDIR)/migrations:/migrations" \
	--network $(COMPOSE_NETWORK) \
	$(MIGRATE_IMAGE) \
	-path=/migrations \
	-database "$(DATABASE_URL_DOCKER)"

.DEFAULT_GOAL := help

.PHONY: help
.PHONY: dev run build generate test check
.PHONY: infra-up infra-down infra-status infra-logs infra-reset
.PHONY: db-migrate db-rollback

help: ## Show this help.
	@awk '\
		BEGIN { FS = ":.*## "; print "Usage: make <target>" } \
		/^[a-zA-Z0-9_-]+:.*## / { printf "  %-14s %s\n", $$1, $$2 } \
	' $(MAKEFILE_LIST)

dev: ## Start PostgreSQL, regenerate views, and run the web server.
	$(MAKE) infra-up
	$(MAKE) generate
	$(GO_RUN) serve

run: ## Run the web server from source.
	$(GO_RUN) serve

build: generate ## Build the release binary into dist/hosthalla.
	mkdir -p $(DIST_DIR)
	$(GO_BUILD) -o $(HOSTHALLA_BIN) $(APP)

generate: ## Regenerate Templ views.
	$(GO) tool templ generate

test: generate ## Regenerate Templ views and run Go tests.
	$(GO) test ./...

check: test ## Regenerate code and run tests.

infra-up: ## Start local PostgreSQL and wait until it is healthy.
	$(COMPOSE) up --detach --wait
	@echo "PostgreSQL: $(DATABASE_URL_LOCAL)"

infra-down: ## Stop local PostgreSQL.
	$(COMPOSE) down

infra-status: ## Show local infrastructure status.
	$(COMPOSE) ps

infra-logs: ## Follow local infrastructure logs.
	$(COMPOSE) logs --follow

infra-reset: ## Stop local PostgreSQL and remove its volume.
	$(COMPOSE) down --volumes --remove-orphans

db-migrate: infra-up ## Apply all pending database migrations.
	$(MIGRATE) up

db-rollback: infra-up ## Roll back one database migration.
	$(MIGRATE) down 1
