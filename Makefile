SHELL := /bin/bash
MIGRATE_IMAGE ?= migrate/migrate:v4.18.3
POSTGRES_CONTAINER ?= helmix-postgres
MIGRATE_DATABASE_URL ?= postgres://helmix:helmix@postgres:5432/helmix?sslmode=disable
POSTGRES_BOOTSTRAP_USER ?= helmix
POSTGRES_BOOTSTRAP_DB ?= helmix

.PHONY: dev build test test-e2e lint migrate migrate-down ensure-postgres-bootstrap wait-postgres logs clean

dev:
	@docker compose up --watch || (echo "compose watch unavailable, starting stack in detached mode" && docker compose up -d)

build:
	go work sync
	go build ./...

test:
	go test ./...
	pytest

test-e2e:
	go test ./tests/e2e/...

lint:
	golangci-lint run ./...
	ruff check .
	eslint .

migrate:
	@$(MAKE) ensure-postgres-bootstrap
	@$(MAKE) wait-postgres
	@NETWORK_NAME="$$(docker inspect -f '{{range $$k, $$v := .NetworkSettings.Networks}}{{$$k}}{{end}}' $(POSTGRES_CONTAINER))"; \
	if [ -z "$$NETWORK_NAME" ]; then \
		echo "could not determine docker network for $(POSTGRES_CONTAINER)"; \
		exit 1; \
	fi; \
	docker run --rm \
		--network "$$NETWORK_NAME" \
		-v "$(PWD)/infra/migrations:/migrations:ro" \
		$(MIGRATE_IMAGE) \
		-path /migrations \
		-database "$(MIGRATE_DATABASE_URL)" \
		up

migrate-down:
	@$(MAKE) ensure-postgres-bootstrap
	@$(MAKE) wait-postgres
	@NETWORK_NAME="$$(docker inspect -f '{{range $$k, $$v := .NetworkSettings.Networks}}{{$$k}}{{end}}' $(POSTGRES_CONTAINER))"; \
	if [ -z "$$NETWORK_NAME" ]; then \
		echo "could not determine docker network for $(POSTGRES_CONTAINER)"; \
		exit 1; \
	fi; \
	docker run --rm \
		--network "$$NETWORK_NAME" \
		-v "$(PWD)/infra/migrations:/migrations:ro" \
		$(MIGRATE_IMAGE) \
		-path /migrations \
		-database "$(MIGRATE_DATABASE_URL)" \
		down 1

ensure-postgres-bootstrap:
	@echo "ensuring postgres role/database bootstrap exists..."
	@for i in {1..60}; do \
		docker exec $(POSTGRES_CONTAINER) pg_isready -U $(POSTGRES_BOOTSTRAP_USER) -d $(POSTGRES_BOOTSTRAP_DB) >/dev/null 2>&1 && break; \
		sleep 2; \
	done; \
	docker exec $(POSTGRES_CONTAINER) psql -U $(POSTGRES_BOOTSTRAP_USER) -d $(POSTGRES_BOOTSTRAP_DB) -tAc "SELECT 1 FROM pg_roles WHERE rolname='helmix'" | grep -q 1 || \
		docker exec $(POSTGRES_CONTAINER) psql -U $(POSTGRES_BOOTSTRAP_USER) -d $(POSTGRES_BOOTSTRAP_DB) -c "CREATE ROLE helmix LOGIN PASSWORD 'helmix';"; \
	docker exec $(POSTGRES_CONTAINER) psql -U $(POSTGRES_BOOTSTRAP_USER) -d $(POSTGRES_BOOTSTRAP_DB) -tAc "SELECT 1 FROM pg_database WHERE datname='helmix'" | grep -q 1 || \
		docker exec $(POSTGRES_CONTAINER) psql -U $(POSTGRES_BOOTSTRAP_USER) -d $(POSTGRES_BOOTSTRAP_DB) -c "CREATE DATABASE helmix OWNER helmix;"; \
	echo "postgres bootstrap ensured"

wait-postgres:
	@echo "waiting for postgres to accept helmix role connections..."
	@for i in {1..60}; do \
		docker exec $(POSTGRES_CONTAINER) pg_isready -U helmix -d helmix >/dev/null 2>&1 && \
		docker exec $(POSTGRES_CONTAINER) psql -U helmix -d helmix -c "select 1;" >/dev/null 2>&1 && \
		echo "postgres is ready" && exit 0; \
		sleep 2; \
	done; \
	echo "postgres did not become ready for helmix role in time"; \
	exit 1

logs:
	@if [ -z "$(service)" ]; then echo "Usage: make logs service=<name>"; exit 1; fi
	docker compose logs -f $(service)

clean:
	docker compose down -v
