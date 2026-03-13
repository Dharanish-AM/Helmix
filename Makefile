SHELL := /bin/bash
MIGRATE_IMAGE ?= migrate/migrate:v4.18.3
POSTGRES_CONTAINER ?= helmix-postgres
MIGRATE_DATABASE_URL ?= postgres://helmix:helmix@postgres:5432/helmix?sslmode=disable
POSTGRES_BOOTSTRAP_USER ?= helmix
POSTGRES_BOOTSTRAP_DB ?= helmix

.PHONY: dev build test test-e2e test-e2e-phase1 test-e2e-phase2-infra test-e2e-phase2-pipeline test-e2e-phase2-deploy test-e2e-phase2-flow test-e2e-phase3-observability test-e2e-phase3-incident lint migrate migrate-down ensure-postgres-bootstrap wait-postgres jwt-keys logs clean

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

test-e2e-phase1:
	@docker compose up -d postgres redis nats jwt-keys auth-service repo-analyzer api-gateway
	@NETWORK_NAME="$$(docker inspect -f '{{range $$k, $$v := .NetworkSettings.Networks}}{{$$k}}{{end}}' $(POSTGRES_CONTAINER))"; \
	if [ -z "$$NETWORK_NAME" ]; then \
		echo "could not determine docker network for $(POSTGRES_CONTAINER)"; \
		exit 1; \
	fi; \
	docker run --rm \
		--network "$$NETWORK_NAME" \
		-v "$(PWD):/workspace" \
		-w /workspace/tests/e2e \
		-e E2E_DATABASE_URL="postgres://helmix:helmix@postgres:5432/helmix?sslmode=disable" \
		-e E2E_API_BASE_URL="http://api-gateway:8080" \
		-e E2E_NATS_URL="nats://nats:4222" \
		-e E2E_JWT_PRIVATE_KEY_PATH="/workspace/certs/jwt-private.pem" \
		golang:1.23 \
		go test . -run TestPhase1AnalyzeViaGatewayPublishesEvent -count=1 -v

test-e2e-phase2-infra:
	@docker compose up -d postgres redis nats jwt-keys auth-service repo-analyzer infra-generator api-gateway
	@NETWORK_NAME="$$(docker inspect -f '{{range $$k, $$v := .NetworkSettings.Networks}}{{$$k}}{{end}}' $(POSTGRES_CONTAINER))"; \
	if [ -z "$$NETWORK_NAME" ]; then \
		echo "could not determine docker network for $(POSTGRES_CONTAINER)"; \
		exit 1; \
	fi; \
	docker run --rm \
		--network "$$NETWORK_NAME" \
		-v "$(PWD):/workspace" \
		-w /workspace/tests/e2e \
		-e E2E_API_BASE_URL="http://api-gateway:8080" \
		-e E2E_JWT_PRIVATE_KEY_PATH="/workspace/certs/jwt-private.pem" \
		golang:1.23 \
		go test . -run TestPhase2GatewayInfraGenerateAuthorized -count=1 -v

test-e2e-phase2-pipeline:
	@docker compose up -d postgres redis nats jwt-keys auth-service repo-analyzer infra-generator pipeline-generator api-gateway
	@NETWORK_NAME="$$(docker inspect -f '{{range $$k, $$v := .NetworkSettings.Networks}}{{$$k}}{{end}}' $(POSTGRES_CONTAINER))"; \
	if [ -z "$$NETWORK_NAME" ]; then \
		echo "could not determine docker network for $(POSTGRES_CONTAINER)"; \
		exit 1; \
	fi; \
	docker run --rm \
		--network "$$NETWORK_NAME" \
		-v "$(PWD):/workspace" \
		-w /workspace/tests/e2e \
		-e E2E_API_BASE_URL="http://api-gateway:8080" \
		-e E2E_JWT_PRIVATE_KEY_PATH="/workspace/certs/jwt-private.pem" \
		golang:1.23 \
		go test . -run TestPhase2GatewayPipelineGenerateAuthorized -count=1 -v

test-e2e-phase2-deploy:
	@docker compose up -d postgres redis nats jwt-keys auth-service repo-analyzer infra-generator pipeline-generator deployment-engine api-gateway
	@NETWORK_NAME="$$(docker inspect -f '{{range $$k, $$v := .NetworkSettings.Networks}}{{$$k}}{{end}}' $(POSTGRES_CONTAINER))"; \
	if [ -z "$$NETWORK_NAME" ]; then \
		echo "could not determine docker network for $(POSTGRES_CONTAINER)"; \
		exit 1; \
	fi; \
	docker run --rm \
		--network "$$NETWORK_NAME" \
		-v "$(PWD):/workspace" \
		-w /workspace/tests/e2e \
		-e E2E_DATABASE_URL="postgres://helmix:helmix@postgres:5432/helmix?sslmode=disable" \
		-e E2E_API_BASE_URL="http://api-gateway:8080" \
		-e E2E_JWT_PRIVATE_KEY_PATH="/workspace/certs/jwt-private.pem" \
		golang:1.23 \
		go test . -run TestPhase2GatewayDeploymentRollbackAuthorized -count=1 -v

test-e2e-phase2-flow:
	@docker compose up -d postgres redis nats jwt-keys auth-service repo-analyzer infra-generator pipeline-generator deployment-engine api-gateway
	@NETWORK_NAME="$$(docker inspect -f '{{range $$k, $$v := .NetworkSettings.Networks}}{{$$k}}{{end}}' $(POSTGRES_CONTAINER))"; \
	if [ -z "$$NETWORK_NAME" ]; then \
		echo "could not determine docker network for $(POSTGRES_CONTAINER)"; \
		exit 1; \
	fi; \
	docker run --rm \
		--network "$$NETWORK_NAME" \
		-v "$(PWD):/workspace" \
		-w /workspace/tests/e2e \
		-e E2E_DATABASE_URL="postgres://helmix:helmix@postgres:5432/helmix?sslmode=disable" \
		-e E2E_API_BASE_URL="http://api-gateway:8080" \
		-e E2E_JWT_PRIVATE_KEY_PATH="/workspace/certs/jwt-private.pem" \
		golang:1.23 \
		go test . -run TestPhase2AnalyzeInfraPipelineDeployFlow -count=1 -v

test-e2e-phase3-observability:
	@docker compose up -d postgres redis nats jwt-keys auth-service repo-analyzer infra-generator pipeline-generator deployment-engine observability api-gateway
	@NETWORK_NAME="$$(docker inspect -f '{{range $$k, $$v := .NetworkSettings.Networks}}{{$$k}}{{end}}' $(POSTGRES_CONTAINER))"; \
	if [ -z "$$NETWORK_NAME" ]; then \
		echo "could not determine docker network for $(POSTGRES_CONTAINER)"; \
		exit 1; \
	fi; \
	docker run --rm \
		--network "$$NETWORK_NAME" \
		-v "$(PWD):/workspace" \
		-w /workspace/tests/e2e \
		-e E2E_DATABASE_URL="postgres://helmix:helmix@postgres:5432/helmix?sslmode=disable" \
		-e E2E_API_BASE_URL="http://api-gateway:8080" \
		-e E2E_NATS_URL="nats://nats:4222" \
		-e E2E_JWT_PRIVATE_KEY_PATH="/workspace/certs/jwt-private.pem" \
		golang:1.23 \
			go test . -run 'TestPhase3Observability' -count=1 -v

test-e2e-phase3-incident:
	@docker compose up -d --force-recreate postgres redis nats jwt-keys auth-service repo-analyzer infra-generator pipeline-generator deployment-engine observability incident-ai api-gateway
	@NETWORK_NAME="$$(docker inspect -f '{{range $$k, $$v := .NetworkSettings.Networks}}{{$$k}}{{end}}' $(POSTGRES_CONTAINER))"; \
	if [ -z "$$NETWORK_NAME" ]; then \
		echo "could not determine docker network for $(POSTGRES_CONTAINER)"; \
		exit 1; \
	fi; \
	docker run --rm \
		--network "$$NETWORK_NAME" \
		-v "$(PWD):/workspace" \
		-w /workspace/tests/e2e \
		-e E2E_DATABASE_URL="postgres://helmix:helmix@postgres:5432/helmix?sslmode=disable" \
		-e E2E_API_BASE_URL="http://api-gateway:8080" \
		-e E2E_NATS_URL="nats://nats:4222" \
		-e E2E_JWT_PRIVATE_KEY_PATH="/workspace/certs/jwt-private.pem" \
		golang:1.23 \
			go test . -run 'TestPhase3Incident' -count=1 -v

lint:
	golangci-lint run ./...
	ruff check .
	eslint .

migrate:
	@docker compose up -d postgres
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
	@docker compose up -d postgres
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

jwt-keys:
	@./scripts/generate-jwt-keys.sh ./certs

logs:
	@if [ -z "$(service)" ]; then echo "Usage: make logs service=<name>"; exit 1; fi
	docker compose logs -f $(service)

clean:
	docker compose down -v
