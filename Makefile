SHELL := /bin/bash
MIGRATE_IMAGE ?= migrate/migrate:v4.18.3
POSTGRES_CONTAINER ?= helmix-postgres
MIGRATE_DATABASE_URL ?= postgres://helmix:helmix@postgres:5432/helmix?sslmode=disable
POSTGRES_BOOTSTRAP_USER ?= helmix
POSTGRES_BOOTSTRAP_DB ?= helmix
TERRAFORM_IMAGE ?= hashicorp/terraform:1.7.5
TF_ENV ?= $(if $(env),$(env),dev)
TF_ENV_DIR ?= infra/terraform/environments/$(TF_ENV)
GOLANGCI_LINT_IMAGE ?= golangci/golangci-lint:v1.61.0
PYTHON_LINT_IMAGE ?= python:3.12-slim
NODE_LINT_IMAGE ?= node:20-alpine
GO_TEST_MODULES ?= \
	libs/auth \
	libs/event-sdk \
	libs/shared-utils \
	services/api-gateway \
	services/auth-service \
	services/repo-analyzer \
	services/infra-generator \
	services/pipeline-generator \
	services/deployment-engine \
	services/observability \
	cli/helmix-cli
GO_LINT_MODULES ?= $(GO_TEST_MODULES) tests/e2e

.PHONY: dev build test test-e2e test-e2e-phase1 test-e2e-phase2-infra test-e2e-phase2-pipeline test-e2e-phase2-deploy test-e2e-phase2-flow test-e2e-phase3-observability test-e2e-phase3-incident test-e2e-phase4-vault cli-build lint lint-go lint-python lint-frontend security-scan-trivy tf-plan tf-apply tf-destroy migrate migrate-down ensure-postgres-bootstrap wait-postgres jwt-keys logs clean

dev:
	@docker compose up --watch || (echo "compose watch unavailable, starting stack in detached mode" && docker compose up -d)

build:
	go work sync
	go build ./...

test:
	@set -euo pipefail; \
	for module in $(GO_TEST_MODULES); do \
		echo "==> go test $$module"; \
		(cd $$module && go test ./...); \
	done
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace/services/incident-ai \
		python:3.12-slim \
		sh -c "pip install --no-cache-dir -r requirements.txt >/tmp/pip.log && pytest tests -q"

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

test-e2e-phase4-vault:
	@docker compose up -d --force-recreate postgres redis nats vault vault-bootstrap jwt-keys auth-service repo-analyzer infra-generator pipeline-generator deployment-engine observability incident-ai api-gateway
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
			go test . -run TestPhase4VaultSecretsCRUDViaGateway -count=1 -v

cli-build:
	@mkdir -p dist
	@cd cli/helmix-cli && \
		GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o ../../dist/helmix-linux-amd64 ./cmd/helmix && \
		GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o ../../dist/helmix-darwin-arm64 ./cmd/helmix && \
		GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o ../../dist/helmix-windows-amd64.exe ./cmd/helmix

lint: lint-go lint-python lint-frontend

lint-go:
	@set -euo pipefail; \
	for module in $(GO_LINT_MODULES); do \
		echo "==> golangci-lint $$module"; \
		docker run --rm \
			-v "$(PWD):/workspace" \
			-w /workspace/$$module \
			$(GOLANGCI_LINT_IMAGE) \
			golangci-lint run ./...; \
	done

lint-python:
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace \
		$(PYTHON_LINT_IMAGE) \
		sh -c "pip install --no-cache-dir ruff >/tmp/pip.log && ruff check services/incident-ai"

lint-frontend:
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace/frontend/dashboard \
		$(NODE_LINT_IMAGE) \
		sh -c "npm ci >/tmp/npm-ci.log && npm run lint"

security-scan-trivy:
	@docker run --rm \
		-v "$(PWD):/src" \
		-w /src \
		aquasec/trivy:0.63.0 fs \
		--scanners vuln,misconfig,secret \
		--severity HIGH,CRITICAL \
		--skip-version-check \
		--ignorefile .trivyignore \
		--ignore-unfixed \
		--exit-code 0 \
		.

tf-plan:
	@if [ ! -d "$(TF_ENV_DIR)" ]; then \
		echo "terraform environment not found: $(TF_ENV_DIR)"; \
		exit 1; \
	fi
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace/$(TF_ENV_DIR) \
		$(TERRAFORM_IMAGE) init -input=false
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace/$(TF_ENV_DIR) \
		$(TERRAFORM_IMAGE) plan -input=false -out=plan.tfplan

tf-apply:
	@if [ "$(APPROVED)" != "true" ]; then \
		echo "manual approval required: rerun with APPROVED=true"; \
		exit 1; \
	fi
	@if [ ! -d "$(TF_ENV_DIR)" ]; then \
		echo "terraform environment not found: $(TF_ENV_DIR)"; \
		exit 1; \
	fi
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace/$(TF_ENV_DIR) \
		$(TERRAFORM_IMAGE) init -input=false
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace/$(TF_ENV_DIR) \
		$(TERRAFORM_IMAGE) apply -input=false -auto-approve plan.tfplan

tf-destroy:
	@if [ "$(TF_ENV)" != "dev" ]; then \
		echo "tf-destroy is limited to env=dev"; \
		exit 1; \
	fi
	@if [ "$(APPROVED)" != "true" ]; then \
		echo "manual approval required: rerun with APPROVED=true env=dev"; \
		exit 1; \
	fi
	@if [ ! -d "$(TF_ENV_DIR)" ]; then \
		echo "terraform environment not found: $(TF_ENV_DIR)"; \
		exit 1; \
	fi
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace/$(TF_ENV_DIR) \
		$(TERRAFORM_IMAGE) init -input=false
	@docker run --rm \
		-v "$(PWD):/workspace" \
		-w /workspace/$(TF_ENV_DIR) \
		$(TERRAFORM_IMAGE) destroy -input=false -auto-approve

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
