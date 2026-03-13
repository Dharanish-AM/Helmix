# ⬡ HELMIX — Autonomous DevOps Platform
### Full Implementation Guide

> *Connect your repo. Everything else is automatic.*

**v1.0 · 12 Weeks · March 2026**

---

## Table of Contents

1. [What Is Helmix?](#1-what-is-helmix)
2. [System Architecture](#2-system-architecture)
3. [Full Tech Stack](#3-full-tech-stack)
4. [Database Schema](#4-database-schema-postgresql-16)
5. [Environment Variables](#5-environment-variables)
6. [Phase-wise Build Roadmap](#6-phase-wise-build-roadmap)
7. [Phase 0 — Foundation](#7-phase-0--foundation)
8. [Phase 1 — GitHub Integration & Foundation Services](#8-phase-1--github-integration--foundation-services)
9. [Phase 2 — Infrastructure Generator, Pipelines & Deployment](#9-phase-2--infrastructure-generator-pipelines--deployment)
10. [Phase 3 — Observability & AI Incident Engine](#10-phase-3--observability--ai-incident-engine)
11. [Phase 4 — Production Hardening](#11-phase-4--production-hardening)
12. [Dashboard — All Pages](#12-dashboard--all-pages)
13. [Full Testing Strategy](#13-full-testing-strategy)
14. [CLI Reference](#14-cli-reference)
15. [Local Development Quick Start](#15-local-development-quick-start)
16. [Production Deployment](#16-production-deployment)
17. [Coding Standards](#17-coding-standards)
18. [Vibe Coding Golden Rules](#18-vibe-coding-golden-rules)

---

## 1. What Is Helmix?

Helmix is an **autonomous DevOps platform**. A developer connects a GitHub repository and Helmix automatically handles everything: detecting the tech stack, generating production-grade infrastructure, creating CI/CD pipelines, deploying to Kubernetes, monitoring the application in real time, and fixing incidents using AI — without any manual intervention.

### Core Automation Pipeline

```
GitHub Repo → Analyze Stack → Generate Infrastructure → Create CI/CD Pipeline
            → Deploy to Kubernetes → Monitor → AI Incident Detection → Auto-Heal → Repeat
```

### 1.1 The Problem It Solves

Every software team faces the same tax: weeks of DevOps setup before a single line of business logic ships. Developers configure servers, write deployment scripts, set up monitoring, and get paged at 3am for incidents they have to diagnose manually. Helmix eliminates this entirely.

| Without Helmix | With Helmix |
|---|---|
| 2–4 weeks of infrastructure setup | < 5 minutes — repo connected, infra generated |
| Manual CI/CD pipeline authoring | Auto-generated GitHub Actions from stack detection |
| On-call engineers debugging incidents | AI root-cause analysis + auto-remediation |
| Separate tools for logs, metrics, traces | Unified observability, one dashboard |
| Rollbacks require human intervention | Blue-green auto-rollback on failed health checks |
| Secrets scattered across env files | Vault-backed secret management, auto-injected |

---

## 2. System Architecture

Helmix is a microservices platform organised into three planes. Services communicate **exclusively through NATS JetStream events** — no direct HTTP calls between services except through the API gateway.

### 2.1 Three Architecture Planes

| Plane | Services | Responsibility |
|---|---|---|
| **Control Plane** | api-gateway, auth-service, repo-analyzer | Intelligence, orchestration, request routing |
| **Execution Plane** | infra-generator, pipeline-generator, deployment-engine | Generate and execute infrastructure and deployments |
| **Observability Plane** | observability, incident-ai, notification-service | Telemetry collection, AI analysis, auto-healing |

### 2.2 Monorepo Structure

```
helmix/
├── services/
│   ├── api-gateway/           # Go + Chi router
│   ├── auth-service/          # Go + OAuth2/JWT
│   ├── repo-analyzer/         # Go + tree-sitter
│   ├── infra-generator/       # Go + Terraform/Helm templating
│   ├── pipeline-generator/    # Go + GitHub Actions YAML generation
│   ├── deployment-engine/     # Go + client-go (Kubernetes)
│   ├── observability/         # Go + OpenTelemetry
│   └── incident-ai/           # Python + FastAPI + Claude API + Qdrant
├── frontend/
│   └── dashboard/             # Next.js 14 App Router
├── cli/
│   └── helmix-cli/            # Go + Cobra (single binary)
├── infra/
│   ├── terraform/             # Cloud infra modules (AWS/GCP/Azure)
│   ├── kubernetes/            # Platform K8s manifests
│   ├── helm-charts/helmix/    # Self-deployment Helm chart
│   └── migrations/            # golang-migrate SQL files
├── libs/
│   ├── event-sdk/             # Shared NATS JetStream event types (Go)
│   ├── auth/                  # Shared JWT middleware (Go)
│   └── shared-utils/          # Logging, tracing helpers (Go)
├── tests/
│   ├── e2e/                   # Playwright + Go integration tests
│   └── load/                  # k6 load test scripts
├── docker-compose.yml         # Local dev stack
├── Makefile                   # All developer commands
└── turbo.json                 # Monorepo pipeline config
```

### 2.3 NATS JetStream Event Flow

| Event | Producer → Consumer(s) |
|---|---|
| `repo.connected` | api-gateway → repo-analyzer |
| `repo.analyzed` | repo-analyzer → infra-generator, pipeline-generator |
| `infra.generated` | infra-generator → deployment-engine |
| `pipeline.created` | pipeline-generator → deployment-engine |
| `deployment.started` | deployment-engine → observability, notification |
| `deployment.succeeded` | deployment-engine → notification |
| `deployment.failed` | deployment-engine → incident-ai, notification |
| `alert.fired` | observability → incident-ai |
| `incident.created` | incident-ai → notification |
| `incident.resolved` | incident-ai → notification |
| `autoheal.triggered` | incident-ai → deployment-engine, notification |

---

## 3. Full Tech Stack

### 3.1 Backend Services

| Layer | Technology | Reason |
|---|---|---|
| Primary language | Go 1.23 | Performance, concurrency, single binary deploys |
| AI/ML service | Python 3.12 + FastAPI | LLM ecosystem, Qdrant client, vector ops |
| API gateway router | Go + Chi | Lightweight, fast middleware chaining |
| Message bus | NATS JetStream | Lighter than Kafka for MVP, durable streams |
| Job queue | Asynq (Redis-backed) | Go-native, retries, scheduling, dashboards |
| Auth | JWT RS256 + OAuth2 (GitHub) | Native GitHub integration, stateless tokens |
| Database | PostgreSQL 16 | ACID, JSONB flexibility, mature ecosystem |
| Cache | Redis 7 | Rate limiting, session storage, job queues |
| Vector DB | Qdrant | Fast ANN search for AI incident memory |
| Secret management | HashiCorp Vault | Dynamic secrets, rotation, audit logs |
| Container registry | GHCR (or Harbor) | GitHub-native, free for public repos |

### 3.2 Infrastructure & DevOps

| Layer | Technology |
|---|---|
| Container orchestration | Kubernetes 1.29 (k3d for local, EKS/GKE/AKS for cloud) |
| Infrastructure as Code | Terraform 1.7 + HCL modules |
| Helm charts | Helm 3 (Helmix ships its own chart) |
| CI/CD engine | Tekton Pipelines (internal) + GitHub Actions (generated) |
| GitOps | ArgoCD for production sync |
| Observability stack | OpenTelemetry → Prometheus + Grafana + Loki + Jaeger |
| LLM provider | `claude-sonnet-4-6` (default) / GPT-4o / Ollama (local) |
| K8s client (Go) | client-go + envtest for integration tests |
| Deployment strategy | Blue-green (default) + Argo Rollouts (canary) |

### 3.3 Frontend & CLI

| Layer | Technology |
|---|---|
| Framework | Next.js 14 App Router |
| Styling | Tailwind CSS + shadcn/ui |
| State management | Zustand (auth) + React Query (server state) |
| Real-time | WebSockets via native WS (with exponential-backoff reconnect) |
| Charts | Recharts |
| CLI framework | Go + Cobra + Viper |
| CLI output styling | charmbracelet/lipgloss |
| CLI output formats | `--output=table` (default) \| `json` \| `yaml` |
| CLI build targets | linux/amd64, darwin/arm64, windows/amd64 |
| Monorepo tooling | Turborepo |
| Testing | Go test + Testify, Pytest, Playwright (E2E) |
| Linting | golangci-lint, Ruff + mypy, ESLint |
| Load testing | k6 |

---

## 4. Database Schema (PostgreSQL 16)

### 4.1 Core Tables

```sql
-- Users & Organizations
CREATE TABLE users (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  github_id   BIGINT UNIQUE NOT NULL,
  username    TEXT NOT NULL,
  email       TEXT NOT NULL,
  avatar_url  TEXT,
  created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE organizations (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name       TEXT NOT NULL,
  slug       TEXT UNIQUE NOT NULL,
  owner_id   UUID REFERENCES users(id),
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE org_members (
  org_id  UUID REFERENCES organizations(id),
  user_id UUID REFERENCES users(id),
  role    TEXT NOT NULL,   -- owner|admin|developer|viewer
  PRIMARY KEY (org_id, user_id)
);

-- Projects & Repos
CREATE TABLE projects (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id     UUID REFERENCES organizations(id),
  name       TEXT NOT NULL,
  slug       TEXT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE(org_id, slug)
);

CREATE TABLE repos (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id     UUID REFERENCES projects(id),
  github_repo    TEXT NOT NULL,   -- 'owner/repo'
  default_branch TEXT DEFAULT 'main',
  detected_stack JSONB,           -- {runtime, framework, database[], ...}
  connected_at   TIMESTAMPTZ DEFAULT now()
);

-- Deployments & Pipelines
CREATE TABLE deployments (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  repo_id     UUID REFERENCES repos(id),
  commit_sha  TEXT NOT NULL,
  branch      TEXT NOT NULL,
  status      TEXT NOT NULL,   -- pending|building|deploying|live|failed|rolled_back
  environment TEXT NOT NULL,   -- dev|staging|prod
  image_tag   TEXT,
  deployed_at TIMESTAMPTZ,
  created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE pipelines (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  repo_id      UUID REFERENCES repos(id),
  run_id       TEXT NOT NULL,
  status       TEXT NOT NULL,
  stages       JSONB,   -- [{name, status, duration_ms, logs_url}]
  triggered_by TEXT,   -- push|manual|schedule
  created_at   TIMESTAMPTZ DEFAULT now()
);

-- Incidents & Alerts
CREATE TABLE alerts (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id  UUID REFERENCES projects(id),
  severity    TEXT NOT NULL,       -- info|warning|critical
  title       TEXT NOT NULL,
  description TEXT,
  status      TEXT DEFAULT 'open', -- open|acknowledged|resolved
  created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE incidents (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  alert_id     UUID REFERENCES alerts(id),
  project_id   UUID REFERENCES projects(id),
  ai_diagnosis JSONB,   -- {root_cause, confidence, reasoning}
  ai_actions   JSONB,   -- [{action, status, timestamp}]
  resolved_at  TIMESTAMPTZ,
  created_at   TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE infra_resources (
  id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID REFERENCES projects(id),
  type       TEXT NOT NULL,         -- deployment|service|ingress|hpa|secret
  name       TEXT NOT NULL,
  namespace  TEXT NOT NULL,
  manifest   JSONB,
  status     TEXT DEFAULT 'pending',
  created_at TIMESTAMPTZ DEFAULT now()
);
```

---

## 5. Environment Variables

```bash
# ── Auth ─────────────────────────────────────────────
GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=
JWT_PRIVATE_KEY_PATH=./certs/jwt-private.pem
JWT_PUBLIC_KEY_PATH=./certs/jwt-public.pem

# ── Database & Cache ─────────────────────────────────
DATABASE_URL=postgres://helmix:helmix@localhost:5432/helmix
REDIS_URL=redis://localhost:6379
NATS_URL=nats://localhost:4222

# ── Kubernetes ───────────────────────────────────────
KUBECONFIG=~/.kube/config
HELMIX_NAMESPACE=helmix-system

# ── AI / LLM ─────────────────────────────────────────
HELMIX_LLM_PROVIDER=anthropic    # anthropic | openai | ollama
ANTHROPIC_API_KEY=
OPENAI_API_KEY=
OLLAMA_URL=http://localhost:11434

# ── Vector DB ────────────────────────────────────────
QDRANT_URL=http://localhost:6333
QDRANT_COLLECTION=helmix-incidents

# ── Container Registry ───────────────────────────────
REGISTRY_URL=ghcr.io/your-org
REGISTRY_USERNAME=
REGISTRY_PASSWORD=

# ── Vault ────────────────────────────────────────────
VAULT_ADDR=http://localhost:8200
VAULT_TOKEN=

# ── Observability ────────────────────────────────────
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
PROMETHEUS_URL=http://localhost:9090
LOKI_URL=http://localhost:3100
JAEGER_URL=http://localhost:16686
```

---

## 6. Phase-wise Build Roadmap

Each phase produces a **working, deployable vertical slice**. Never start Phase N+1 until all acceptance tests in Phase N pass. The phases build on each other — every service built in Phase 1 is used by Phase 2.

| Phase | Focus | Timeline |
|---|---|---|
| **Phase 0** | Foundation & Dev Environment | Days 1–3 |
| **Phase 1** | GitHub Integration + Stack Detection + Auth | Weeks 1–3 |
| **Phase 2** | Infrastructure Generator + Pipeline Generator + Deployments | Weeks 4–6 |
| **Phase 3** | Observability + AI Incident Engine + Auto-Heal | Weeks 7–10 |
| **Phase 4** | Production Hardening: RBAC · Vault · Terraform · Security | Weeks 11–12 |

> **How to use the phase prompts:** Each task below includes an exact prompt to paste into your AI coding assistant (Cursor, Windsurf, Claude Code, GitHub Copilot Workspace).
> - Paste the **full prompt** — do not summarise it. Context is critical.
> - Complete each task, run its tests, commit, then move to the next.
> - If a test fails, paste the error back into the AI and ask it to fix.
> - **Never skip a test suite.** Broken foundations break everything above.

---

## 7. Phase 0 — Foundation

> **Goal:** One command (`make dev`) starts the entire local stack. Nothing else starts until this works.

### Phase 0 · Prompt 0.1 — Monorepo Bootstrap

```
You are bootstrapping a production Go monorepo called 'helmix'.

Create this exact folder structure:
  helmix/                (root with go.work for multi-module workspace)
  services/              (one subdirectory per service, each with own go.mod)
  libs/event-sdk/, libs/auth/, libs/shared-utils/  (shared Go modules)
  frontend/dashboard/    (Next.js 14 — scaffold with: npx create-next-app@latest)
  cli/helmix-cli/        (Go module)
  infra/migrations/      (golang-migrate SQL files)
  tests/e2e/, tests/load/

Create root Makefile with targets:
  make dev          → docker-compose up --watch
  make build        → go build ./... for all services
  make test         → go test ./... + pytest
  make test-e2e     → run tests/e2e/
  make lint         → golangci-lint + ruff + eslint
  make migrate      → run golang-migrate up
  make migrate-down → golang-migrate down 1
  make logs service=X → docker-compose logs -f X
  make clean        → docker-compose down -v

Create turbo.json for pipeline: lint → test → build.
Create .env.example with all required variables (see Section 5).
Create docker-compose.yml with health checks for:
  postgres:16, redis:7, nats (nats:2.10), qdrant:latest, vault:1.15

Every Go service must have /health returning:
  {status:'ok', service:'name', version:'0.1.0'}
```

### Phase 0 · Prompt 0.2 — Database Migrations

```
Create golang-migrate SQL migration files in infra/migrations/.

Files to create (up + down for each):
  000001_create_users.up.sql
  000002_create_organizations.up.sql
  000003_create_projects_repos.up.sql
  000004_create_deployments_pipelines.up.sql
  000005_create_alerts_incidents.up.sql
  000006_create_infra_resources.up.sql

Use the exact schema from Section 4 of this document.
Add pgcrypto extension in migration 000001.
Add indexes on: repos.project_id, deployments.repo_id,
  incidents.project_id, alerts.project_id.
Add Makefile target: make migrate (runs all up migrations).
Test by running make migrate on a fresh postgres container.
```

### Phase 0 · Prompt 0.3 — libs/event-sdk

```
Create libs/event-sdk/ as a Go module.

File: libs/event-sdk/events.go

Define these types exactly:
  type EventType string
  const (
    RepoConnected, RepoAnalyzed, InfraGenerated,
    PipelineCreated, DeploymentStarted, DeploymentSucceeded,
    DeploymentFailed, AlertFired, IncidentCreated,
    IncidentResolved, AutoHealTriggered EventType = '...'
  )

  type BaseEvent struct {
    ID, Type, OrgID, ProjectID string
    CreatedAt time.Time
  }

  type DetectedStack struct {
    Runtime, Framework string
    Database           []string
    Containerized, HasTests bool
    Port               int
    BuildCommand, TestCommand string
  }

  type RepoAnalyzedEvent   struct { BaseEvent; RepoID string; Stack DetectedStack }
  type DeploymentEvent     struct { BaseEvent; DeploymentID, CommitSHA, Environment, ImageTag string }
  type AlertFiredEvent     struct { BaseEvent; AlertID, Severity, Metric string; Value, Threshold float64 }

Add a NATS publisher helper:
  func Publish(nc *nats.Conn, event interface{}) error

Add a NATS subscriber helper:
  func Subscribe[T any](nc *nats.Conn, subject string, handler func(T)) error

Write unit tests: libs/event-sdk/events_test.go
```

### Phase 0 — Acceptance Criteria

- `make dev` starts all containers with no errors
- All health checks pass (`docker ps` shows all healthy)
- `make migrate` runs successfully on fresh database
- All 8 tables exist with correct columns (`psql \dt` to verify)
- NATS web UI accessible at `http://localhost:8222`
- Qdrant API accessible at `http://localhost:6333`
- `go test ./...` in `libs/event-sdk` passes

---

## 8. Phase 1 — GitHub Integration & Foundation Services

> **Goal:** GitHub OAuth login, repo connection, intelligent stack detection, and basic dashboard. The core user journey: login → connect repo → see detected stack.

### Phase 1 · Prompt 1.1 — Auth Service

```
Build services/auth-service/ in Go.

This service owns ALL authentication for Helmix.

Endpoints:
  GET  /health                 → {status:'ok', service:'auth-service'}
  GET  /auth/github            → redirect to GitHub OAuth
  GET  /auth/github/callback   → exchange code for access_token,
                                  create/update user in PostgreSQL,
                                  issue JWT (RS256, 24h expiry),
                                  store refresh token in Redis (30d TTL)
  POST /auth/refresh           → accept refresh token, rotate it,
                                  return new JWT + new refresh token
  GET  /auth/me                → return current user (requires JWT)
  POST /auth/logout            → delete refresh token from Redis

JWT payload: { user_id, org_id, role, email, github_username, exp }
JWT algorithm: RS256 (private key at JWT_PRIVATE_KEY_PATH env var)

Store GitHub access tokens encrypted (AES-256-GCM) in postgres users table.
On /callback: if user exists (same github_id) → update, don't duplicate.

Create reusable middleware at libs/auth/middleware.go:
  func JWTMiddleware(publicKeyPath string) func(http.Handler) http.Handler
  func RequireRole(roles ...string) func(http.Handler) http.Handler
  func UserFromContext(ctx context.Context) *User

Use slog for structured JSON logging.
Validate all env vars at startup using a Config struct.
Every error must be wrapped: fmt.Errorf('creating user: %w', err)
```

### Phase 1 · Prompt 1.2 — API Gateway

```
Build services/api-gateway/ in Go using Chi router.

This is the single entry point for all Helmix API traffic.

Middleware stack (applied in this order):
  1. RequestID  — inject X-Request-ID (generate if missing)
  2. Logger     — structured JSON log per request (method, path, status, latency)
  3. OTel       — OpenTelemetry trace per request
  4. Auth       — JWT validation via libs/auth/middleware (skip /auth/* routes)
  5. RateLimit  — 100 req/min per user_id via Redis sliding window

Routes (proxy to downstream services):
  /api/v1/auth/*           → auth-service:8081
  /api/v1/repos/*          → repo-analyzer:8082
  /api/v1/infra/*          → infra-generator:8083
  /api/v1/pipelines/*      → pipeline-generator:8084
  /api/v1/deployments/*    → deployment-engine:8085
  /api/v1/observability/*  → observability:8086
  /api/v1/incidents/*      → incident-ai:8087
  /ws/*                    → proxy WebSocket connections (for live updates)

Standard error envelope for ALL errors:
  { error: string, code: string, request_id: string }

Rate limit response:    429 with Retry-After header.
Service unavailable:    503 with error envelope.
Config struct validates all env vars at startup.
```

### Phase 1 · Prompt 1.3 — Repo Analyzer Service

```
Build services/repo-analyzer/ in Go.

This service detects tech stacks from GitHub repositories.

Endpoint: POST /analyze
Body: { repo_url: string, github_token: string, repo_id: string }

Detection algorithm (check files in this exact order):
  1. package.json → look at dependencies for:
       'next'         → framework='nextjs',  runtime='node'
       'react' (no next) → framework='react', runtime='node'
       'express'      → framework='express', runtime='node'
       '@nestjs/core' → framework='nestjs',  runtime='node'
       Check 'engines.node' for node version

  2. requirements.txt / pyproject.toml → look for:
       'django'  → framework='django',  runtime='python'
       'fastapi' → framework='fastapi', runtime='python'
       'flask'   → framework='flask',   runtime='python'

  3. pom.xml → 'spring-boot' → framework='spring', runtime='java'

  4. go.mod → look at require block for:
       'gin-gonic/gin'  → framework='gin',   runtime='go'
       'labstack/echo'  → framework='echo',  runtime='go'
       'gofiber/fiber'  → framework='fiber', runtime='go'

  5. Gemfile → 'rails' → framework='rails', runtime='ruby'
  6. Dockerfile present → containerized=true
  7. docker-compose.yml → extract image names for database hints
  8. .env.example → scan for postgres://, mysql://, mongodb://, redis://

If confidence < 0.70 (unknown stack): call incident-ai /classify
  with a sample of file paths and contents for LLM classification.

After detection: publish NATS event repo.analyzed (libs/event-sdk)
Update postgres repos.detected_stack with result JSONB.
Cleanup: delete cloned temp directory.

Shallow clone: git clone --depth=1 --single-branch {repo_url} /tmp/{repo_id}
Use github_token for auth: https://token:{github_token}@github.com/...
```

### Phase 1 · Prompt 1.4 — Dashboard (Next.js 14)

```
Build frontend/dashboard/ with Next.js 14 App Router.

Pages:
  /         → redirect to /dashboard
  /login    → 'Connect with GitHub' button (links to /api/v1/auth/github)
  /dashboard → protected route; shows:
      - Header: Helmix logo + user avatar + org name
      - 'Connect repository' button → opens modal
      - GitHub repo picker modal: search GitHub repos, select, confirm
      - List of connected repos (name, stack badge, status, last deploy)
      - Empty state illustration when no repos

Auth flow:
  - On /login, clicking 'Connect with GitHub' → navigate to /api/v1/auth/github
  - After OAuth callback, backend redirects to /dashboard?token=<jwt>
  - Store JWT in Zustand + localStorage
  - All API calls include Authorization: Bearer <token> header
  - If 401 received: clear token, redirect to /login

Stack: Zustand (auth state) + React Query (API calls) + shadcn/ui + Tailwind
Protect routes: middleware.ts with NextAuth or manual JWT check
API base URL from NEXT_PUBLIC_API_URL env var
```

### Phase 1 — Tests

**Auth Service** (`services/auth-service/internal/handler/auth_test.go`):
- `TestGitHubCallbackCreatesUser` — mock GitHub API, assert user in DB, JWT returned
- `TestGitHubCallbackUpdatesExistingUser` — same github_id → update only, no duplicate row
- `TestRefreshTokenRotation` — old token invalidated, new one issued
- `TestExpiredJWTRejected` — token past expiry returns 401
- `TestMeEndpointReturnsUser` — valid JWT → correct user JSON
- Use `testcontainers-go` for real PostgreSQL + real Redis in tests

**Repo Analyzer** (`services/repo-analyzer/internal/analyzer/detect_test.go`):
- `TestDetectNextJS` — fixture dir with package.json containing 'next' dep
- `TestDetectDjango` — fixture dir with requirements.txt containing 'django'
- `TestDetectSpringBoot` — fixture dir with pom.xml containing spring-boot-starter
- `TestDetectGin` — fixture dir with go.mod containing gin-gonic/gin
- `TestDetectContainerized` — fixture dir with Dockerfile present → containerized=true
- `TestDetectPostgresFromEnv` — .env.example with DATABASE_URL=postgres:// → database=['postgres']
- `TestUnknownStack` — empty dir → confidence < 0.70, fallback flag set

**API Gateway** (`services/api-gateway/integration_test.go`):
- `TestRateLimitEnforced` — 101 requests in 60s → 101st returns 429 with Retry-After
- `TestUnauthenticatedRequestRejected` — no JWT → 401 with error envelope
- `TestExpiredJWTRejected` — expired JWT → 401
- `TestRequestIDInjected` — every response has X-Request-ID header
- `TestHealthEndpoint` — GET /health → 200 `{status:'ok'}`

**E2E Test** (`tests/e2e/phase1_test.go`):
1. Start full docker-compose stack
2. Simulate GitHub OAuth callback → receive JWT
3. POST `/api/v1/repos/analyze` with public GitHub repo URL
4. Poll GET `/api/v1/repos/{id}` until status = 'analyzed' (timeout: 60s)
5. Assert `detected_stack` is not empty
6. Assert `repo.analyzed` NATS event was published
7. Run with: `make test-e2e-phase1`

---

## 9. Phase 2 — Infrastructure Generator, Pipelines & Deployment

> **Goal:** From detected stack → auto-generated K8s manifests → auto-generated GitHub Actions CI/CD → blue-green deployment to Kubernetes. The full deploy loop.

### Phase 2 · Prompt 2.1 — Infrastructure Generator

```
Build services/infra-generator/ in Go.
Listens for NATS event: repo.analyzed

For each event, generate these Kubernetes manifests using Go templates
(templates stored in services/infra-generator/templates/*.yaml.tmpl):

  1. namespace.yaml     — isolated namespace: helmix-{org_id}-{project_slug}
  2. deployment.yaml    — with liveness probe (/health), readiness probe (/ready),
                          and resource requests/limits based on runtime:
                            node:   requests: 256m CPU / 512Mi  | limits: 500m / 1Gi
                            python: requests: 512m CPU / 1Gi    | limits: 1000m / 2Gi
                            go:     requests: 128m CPU / 256Mi  | limits: 256m / 512Mi
                            java:   requests: 1000m CPU / 2Gi   | limits: 2000m / 4Gi
  3. service.yaml       — ClusterIP, port from detected_stack.port (default 8080)
  4. ingress.yaml       — Nginx ingress class, TLS via cert-manager annotation,
                          host: {project_slug}.helmix.dev
  5. hpa.yaml           — HPA min=2, max=10, CPU target=70%
  6. secret.yaml        — ExternalSecret CRD (Vault path: secret/{org_id}/{project_id}/)
  7. Dockerfile         — only if detected_stack.containerized=false:
                            node   → FROM node:18-alpine, COPY, RUN npm ci, CMD node
                            python → FROM python:3.12-slim, COPY, RUN pip install, CMD uvicorn
                            go     → multi-stage: golang:1.23 builder + distroless/base

If detected_stack.database contains 'postgres':
  → inject env var DATABASE_URL from secret into deployment
If detected_stack.database contains 'redis':
  → inject env var REDIS_URL from secret

Validate all generated YAML using k8s.io/apimachinery before storing.
Store each manifest in postgres infra_resources table.
Publish NATS event: infra.generated

Expose: GET /infra/{project_id}/manifests → returns all generated YAML
```

### Phase 2 · Prompt 2.2 — Pipeline Generator

```
Build services/pipeline-generator/ in Go.
Listens for NATS event: infra.generated

Generate .github/workflows/helmix-deploy.yml with these 7 stages:

  Stage 1: checkout  — actions/checkout@v4
  Stage 2: setup     — use the correct action based on runtime:
                         node   → actions/setup-node@v4 with node-version from detected_stack
                         python → actions/setup-python@v5
                         go     → actions/setup-go@v5
                         java   → actions/setup-java@v4 with temurin distribution
  Stage 3: test      — run test command from detected_stack.test_command:
                         default per runtime: npm test | pytest | go test ./... | mvn test
  Stage 4: sast-scan — trivy filesystem scan, SARIF output,
                         upload to GitHub Security tab
  Stage 5: docker-build — build image tagged:
                         ghcr.io/{org}/{project}:{github.sha}
                         ghcr.io/{org}/{project}:latest
  Stage 6: docker-push  — push both tags to GHCR
  Stage 7: deploy    — POST to https://api.helmix.dev/v1/deployments
                         with {repo_id, image_tag: '${{ github.sha }}', environment: 'staging'}
                         authenticated with HELMIX_API_TOKEN secret

Commit the generated YAML to the repo via GitHub API:
  → Create a PR: 'Add Helmix CI/CD pipeline'
  → Branch: helmix/add-pipeline

Store generated YAML in postgres pipelines table.
Publish NATS event: pipeline.created
Expose: POST /pipelines/generate → {repo_id, github_token}
Expose: GET /pipelines/{repo_id}/status
```

### Phase 2 · Prompt 2.3 — Deployment Engine

```
Build services/deployment-engine/ in Go using client-go.
This service executes actual deployments to Kubernetes.

Endpoints:
  POST /deploy                      → trigger new deployment
  GET  /deployments/{id}            → deployment status
  POST /deployments/{id}/rollback   → manual rollback

Blue-Green Deployment Algorithm:
  1. Load current deployment (blue) from PostgreSQL
  2. Create new Deployment manifest with name '{name}-green', new image tag
  3. Apply to Kubernetes namespace using client-go
  4. Watch pod readiness: poll every 5s, timeout after 5 minutes
       → if timeout: delete green deployment, update DB status='failed',
                     publish deployment.failed, STOP
  5. When green pods are all Ready:
       → Update Service selector to target green pods
       → Update DB: deployment status='live'
       → Publish deployment.succeeded
  6. After 2 minutes: delete blue deployment pods

Rollback:
  → Revert Service selector to previous deployment
  → Scale previous deployment back up to original replicas
  → Update DB: current='rolled_back', previous='live'
  → Publish deployment.failed with action='rollback'

CRITICAL: Blue-green state must be in PostgreSQL, never in memory.
All timeouts must come from env vars (DEPLOY_TIMEOUT_SECONDS, etc.).
Use context.WithTimeout for all Kubernetes operations.
Load kubeconfig from KUBECONFIG env var.
```

### Phase 2 — Tests

**Infrastructure Generator** (`services/infra-generator/internal/generator/templates_test.go`):
- `TestGenerateNodeDeployment` — Next.js stack → node:18-alpine image, port 3000
- `TestGeneratePythonDeployment` — Django stack → python:3.12-slim, port 8000
- `TestGenerateHPA` — always min=2, max=10, CPU target=70%
- `TestGenerateIngressWithTLS` — ingress has cert-manager.io/cluster-issuer annotation
- `TestInjectDatabaseSecret` — postgres in stack → DATABASE_URL env var in deployment
- `TestGenerateDockerfileWhenNeeded` — containerized=false → Dockerfile generated
- `TestAllManifestsValidK8sYAML` — parse all outputs with k8s.io/apimachinery, no errors

**Pipeline Generator** (`services/pipeline-generator/internal/generator/pipeline_test.go`):
- `TestNodePipelineHasNpmTest` — Next.js stack → 'npm test' in stage 3
- `TestPythonPipelineHasPytest` — Django stack → 'pytest' in stage 3
- `TestTrivyScanPresent` — all generated pipelines include trivy-action step
- `TestDockerBuildHasCorrectTag` — image tag is `ghcr.io/{org}/{project}:{sha}`
- `TestHelmixDeployStepIsLast` — deploy API call is always stage 7
- `TestValidGitHubActionsYAML` — parse YAML, assert 'on', 'jobs' keys present

**Deployment Engine** (`services/deployment-engine/internal/deploy/bluegreen_test.go`):
- `TestBlueGreenHappyPath` — green pods Ready → traffic shifts → blue torn down (uses envtest)
- `TestBlueGreenTimeoutTriggersRollback` — green never Ready → DB status='failed'
- `TestRollbackRestoresPreviousImage` — rollback → Service selector points to previous
- `TestDeploymentStatusAllTransitionsInDB` — all status changes persisted

**E2E Test** (`tests/e2e/phase2_test.go`):
1. Connect a test repo (reuse Phase 1 setup)
2. POST `/infra/generate` → assert K8s manifests in DB
3. POST `/pipelines/generate` → assert GitHub Actions YAML created (mock GitHub API)
4. Simulate pipeline completion: POST `/deploy` with image_tag='sha123'
5. Poll until deployment.status = 'live' (timeout: 2min)
6. POST `/deployments/{id}/rollback`
7. Assert status = 'rolled_back' in DB and previous = 'live'

---

## 10. Phase 3 — Observability & AI Incident Engine

> **Goal:** Collect metrics/logs/traces, detect anomalies, use Claude AI for root-cause analysis, auto-remediate with confidence > 0.85. The most innovative part of the platform.

### Phase 3 · Prompt 3.1 — Observability Service

```
Build services/observability/ in Go.

Responsibilities:
  1. OTEL Gateway: receive spans from all services and forward to Jaeger
  2. Prometheus: scrape /metrics from all deployed pods every 15s
  3. Loki: forward structured logs from all pods
  4. Metric snapshots: write to postgres every 60s per project:
       {project_id, timestamp, cpu_pct, memory_pct, req_per_sec,
        error_rate_pct, p99_latency_ms, pod_count, ready_pod_count}

Alert rules (evaluated every 30s):
  Rule 1: cpu_pct > 85 sustained 5 minutes      → severity=warning
  Rule 2: error_rate_pct > 5 sustained 2 minutes → severity=critical
  Rule 3: p99_latency_ms > 2000 sustained 3 min  → severity=warning
  Rule 4: pod restarts > 3 in last 10 minutes    → severity=critical
  Rule 5: ready_pod_count = 0                     → severity=critical (immediate)

Alert deduplication: same project + same rule = only one open alert.
Do not re-fire until previous alert is resolved.

On alert breach: insert row in postgres alerts table,
  publish NATS event: alert.fired (libs/event-sdk AlertFiredEvent)

Endpoints:
  GET /metrics/{project_id}          → last 24h snapshots (JSON array)
  GET /metrics/{project_id}/current  → live scrape result
  GET /alerts/{project_id}           → open alerts
  Expose /metrics on port 9090 in Prometheus format for Helmix's own monitoring.
```

### Phase 3 · Prompt 3.2 — AI Incident Engine

```
Build services/incident-ai/ in Python 3.12 + FastAPI.
Use: nats-py, anthropic, qdrant-client, pydantic v2, structlog, httpx, asyncio

STEP 1 — Intake: Subscribe to NATS alert.fired events (nats-py JetStream)

STEP 2 — Context gathering (async, parallel):
  a. Fetch last 60min metric snapshots from observability service
  b. Fetch last 500 log lines from Loki (filter ERROR and WARN)
  c. Fetch last 5 deployments from deployment-engine API
  d. Search Qdrant for top 3 similar past incidents (cosine similarity)

STEP 3 — LLM diagnosis. Call the configured LLM provider with this prompt:
---
You are an expert SRE analyzing a production incident.

Alert: {title} (severity: {severity}) at {timestamp}
Project: {project_id}
Metrics (last 60 min): {metrics_summary}
Error logs (last 500 lines): {error_logs}
Recent deployments: {deployment_history}
Similar past incidents: {qdrant_results}

Respond ONLY with valid JSON (no markdown, no preamble):
{
  "root_cause": "one clear sentence",
  "confidence": 0.0-1.0,
  "reasoning": "step by step analysis",
  "recommended_actions": [
    {"action": "scale_pods",           "params": {"replicas": 5}},
    {"action": "rollback_deployment",  "params": {"deployment_id": "..."}},
    {"action": "restart_pods",         "params": {}},
    {"action": "increase_memory_limit","params": {"limit_mb": 1024}}
  ],
  "auto_execute": true
}
---

STEP 4 — Store in postgres incidents table (ai_diagnosis JSONB column)
          Publish NATS event: incident.created

STEP 5 — Auto-remediation (only if auto_execute=true AND confidence >= 0.85):
  Execute recommended_actions in order via internal Helmix APIs:
    scale_pods            → PATCH /deployments/{id}/scale
    rollback_deployment   → POST  /deployments/{id}/rollback
    restart_pods          → POST  /deployments/{id}/restart
    increase_memory_limit → PATCH /deployments/{id}/resources

  If one action fails → try next action in list
  Log each action to incidents.ai_actions JSONB
  After all actions: wait 5 minutes, check if alert still firing
    → resolved:      publish incident.resolved, update incidents.resolved_at
    → still firing:  set auto_execute=false, emit notification for human

STEP 6 — Qdrant memory (on incident resolved):
  Embed text: '{root_cause}. Symptoms: {alert_title}. Fix: {actions_taken}.'
  Store in Qdrant collection 'helmix-incidents' for future retrieval
  On new incident: retrieve top-3 (cosine score > 0.7) as historical context

LLM Provider abstraction (services/incident-ai/llm/provider.py):
  class LLMProvider(ABC):
    async def diagnose(self, prompt: str) -> str: ...
  AnthropicProvider → claude-sonnet-4-6
  OpenAIProvider    → gpt-4o
  OllamaProvider    → local model
  Select via HELMIX_LLM_PROVIDER env var

Retry:    exponential backoff, max 3 attempts, 30s hard timeout
Log:      every prompt+response to audit log file
Auto-heal opt-in: check project settings before auto-executing.
Default: auto-heal disabled. User enables per-project via dashboard.

Endpoints:
  GET  /incidents/{project_id}            → list incidents
  GET  /incidents/{incident_id}           → detail with full diagnosis
  POST /incidents/{incident_id}/actions   → manually trigger an action
  GET  /incidents/{incident_id}/similar   → top 3 similar from Qdrant
  POST /classify                          → LLM stack classification fallback
```

### Phase 3 — Tests

**Observability** (`services/observability/internal/alerting/rules_test.go`):
- `TestCPUAlertFires` — CPU > 85% for 5 consecutive 30s intervals → alert created
- `TestCPUAlertNoFire` — CPU > 85% for only 3 intervals (1.5 min) → no alert
- `TestErrorRateCritical` — error_rate > 5% for 2 min → severity=critical
- `TestZeroPodImmediate` — ready_pod_count=0 → alert fires without waiting
- `TestAlertDeduplication` — same condition fires again → no duplicate alert row

**AI Incident Engine** (`services/incident-ai/tests/test_diagnosis.py`):
- `test_diagnosis_parsed_correctly` — mock LLM returns valid JSON → Pydantic model correct (use respx)
- `test_low_confidence_no_autoexecute` — confidence=0.5 → auto_execute forced false
- `test_qdrant_similar_included` — mock Qdrant returns 2 results → both appear in LLM prompt
- `test_retry_on_llm_timeout` — LLM times out twice, succeeds on third → result returned
- `test_action_rollback_called` — diagnosis recommends rollback → deployment-engine API called

**Auto-Remediation** (`services/incident-ai/tests/test_autoheal.py`):
- `test_scale_pods_action` — scale_pods in actions → PATCH /deployments/{id}/scale called
- `test_action_failure_tries_next` — first action returns 500 → second action attempted
- `test_incident_resolved_after_heal` — alert clears after wait → incident.resolved published
- `test_escalate_on_persistent_alert` — alert still firing after 5min → auto_execute=false

**E2E Test** (`tests/e2e/phase3_test.go`):
1. Inject synthetic high error-rate metrics into observability service
2. Wait for `alert.fired` NATS event (timeout: 2 minutes)
3. Wait for `incident.created` NATS event (timeout: 1 minute)
4. Assert incident has root_cause and confidence > 0 in DB
5. POST `/incidents/{id}/actions` with action=restart_pods
6. Assert `autoheal.triggered` NATS event published
7. Inject normal metrics (error_rate back to 0%)
8. Wait for `incident.resolved` event (timeout: 5 minutes)
9. Assert `incidents.resolved_at` is set in DB

---

## 11. Phase 4 — Production Hardening

> **Goal:** Multi-tenancy RBAC, Vault secret management, Terraform for cloud infra, security hardening, cost dashboard, full CLI. Production-ready.

### Phase 4 · Prompt 4.1 — Multi-tenancy & RBAC

```
Extend auth-service with full multi-tenancy support.

Organization model:
  Users belong to one or more organisations.
  Each org has members with roles: OWNER | ADMIN | DEVELOPER | VIEWER

Permissions matrix:
  OWNER:     all actions in org (create/delete projects, manage members)
  ADMIN:     manage projects, trigger deployments, view incidents
  DEVELOPER: trigger deployments, view all resources
  VIEWER:    read-only on all resources (no write operations)

JWT must now include: user_id, org_id, role, email, github_username

All downstream services must:
  - Extract org_id from JWT
  - Only return resources belonging to that org_id
  - Return 403 if user tries to access another org's resources

New endpoints on auth-service:
  POST   /orgs                        → create org
  POST   /orgs/invite                 → send email invite (Resend API)
  POST   /orgs/accept-invite          → accept invite, join org
  GET    /orgs/members                → list members + roles
  PATCH  /orgs/members/{user_id}      → update role
  DELETE /orgs/members/{user_id}      → remove member

Add RequireRole middleware to libs/auth that checks role from JWT.
```

### Phase 4 · Prompt 4.2 — Vault Integration

```
Integrate HashiCorp Vault for all secret management.
Local dev: Vault in dev mode via docker-compose (already scaffolded in Phase 0).

Changes to infra-generator:
  → Replace K8s Secret manifests with ExternalSecret CRDs
  → ExternalSecret references Vault path: secret/{org_id}/{project_id}/
  → Requires external-secrets-operator installed in cluster

New API endpoint on api-gateway: /api/v1/secrets
  POST   /secrets/{project_id}       → {key, value} stored in Vault
  GET    /secrets/{project_id}       → list secret keys (NOT values)
  DELETE /secrets/{project_id}/{key} → delete a secret

Secrets stored at: secret/data/{org_id}/{project_id}/{key}
NEVER stored in PostgreSQL.

Vault token management:
  - Each service gets a Vault AppRole (roleID + secretID)
  - Tokens auto-renew before expiry
  - Rotation schedule: 24h via a cron job in Kubernetes
  - Add Vault agent sidecar config for production Kubernetes deployments.
```

### Phase 4 · Prompt 4.3 — Terraform Modules

```
Create Terraform modules in infra/terraform/.
All modules must work with a common variable interface across AWS/GCP/Azure.
Use the variable 'cloud_provider' (aws|gcp|azure) to switch implementations.

Module: modules/vpc/
  Variables: cloud_provider, region, cidr_block, az_count
  Outputs:   vpc_id, public_subnet_ids[], private_subnet_ids[]

Module: modules/kubernetes/
  Variables: cloud_provider, region, vpc_id, subnet_ids, node_type, min_nodes, max_nodes
  Creates:   EKS (aws) / GKE (gcp) / AKS (azure)
  Outputs:   cluster_endpoint, cluster_ca, kubeconfig_command

Module: modules/database/
  Variables: cloud_provider, region, vpc_id, subnet_ids, instance_class, storage_gb
  Creates:   RDS PostgreSQL (aws) / Cloud SQL (gcp) / Azure DB (azure)
  Options:   multi_az=true for production
  Outputs:   db_endpoint, db_port, db_name

Module: modules/cache/
  Creates: ElastiCache Redis / Memorystore / Azure Cache
  Outputs: redis_endpoint

Module: modules/registry/
  Creates: ECR / Artifact Registry / ACR
  Outputs: registry_url

Environments:
  environments/dev/main.tf         → uses k3d locally, skip cloud resources
  environments/staging/main.tf    → single-AZ, small instance sizes, no multi-AZ
  environments/production/main.tf → multi-AZ, larger sizes, automated backups

Add to Makefile:
  make tf-plan env=staging    → terraform plan for environment
  make tf-apply env=staging   → terraform apply (requires manual approval in CI)
  make tf-destroy env=dev     → terraform destroy (dev only)
```

### Phase 4 · Prompt 4.4 — Security Hardening

```
Harden the entire Helmix platform for production.

1. Security headers on api-gateway (add middleware):
     Strict-Transport-Security: max-age=63072000; includeSubDomains; preload
     Content-Security-Policy: default-src 'self'
     X-Frame-Options: DENY
     X-Content-Type-Options: nosniff
     Referrer-Policy: strict-origin-when-cross-origin
     Permissions-Policy: camera=(), microphone=(), geolocation=()

2. Input validation middleware on api-gateway:
     - Reject requests with body > 1MB (413)
     - Validate all UUID path parameters (400 if invalid)
     - Strip null bytes from string inputs
     - Return 400 with detailed validation errors

3. Container image scanning:
     - Trivy scans all user-provided images before deployment
     - Block deploy if Trivy finds CRITICAL severity CVEs
     - Store scan results in postgres (add scan_results JSONB to deployments)
     - Override possible via 'accept_risk' flag (ADMIN role only)

4. Kubernetes security:
     - NetworkPolicy: each service can only talk to services it needs
     - Pod Security Standards: restricted PSA on all user namespaces
     - ServiceAccount per service with minimum RBAC (no cluster-admin)
     - Resource quotas per org namespace

5. Auth security:
     - Rate limit /auth/github/callback: 5 attempts per IP per 15 minutes
     - Log all auth events (login, logout, failed attempts) to audit_logs table
     - CORS: only allow helmix.dev origins in production
```

### Phase 4 · Prompt 4.5 — Full CLI

```
Complete cli/helmix-cli/ with all commands using Go + Cobra + Viper.

Commands:
  helmix login                             → OAuth device flow, store JWT in ~/.helmix/config.yaml
  helmix projects list                     → list org's projects (table output)
  helmix projects create --name X          → create new project
  helmix repos connect <github-url>        → connect GitHub repo to project
  helmix repos status [--watch]            → show repo analysis status
  helmix deploy [--env staging|prod] [--image <tag>] [--watch]
  helmix rollback [--deployment <id>]
  helmix logs [--project <id>] [--tail 200] [--since 30m]
  helmix incidents list [--open]
  helmix incidents show <id>               → full diagnosis + actions
  helmix incidents heal <id> [--dry-run]
  helmix metrics [--project <id>] [--watch]
  helmix secrets set KEY=VALUE
  helmix secrets list
  helmix cost [--project <id>]             → estimated monthly cost breakdown
  helmix completion bash|zsh|fish          → shell completion script

Global flags: --output=table|json|yaml  --project <id>  --org <id>

Colors (charmbracelet/lipgloss):
  green  = success / live
  red    = error / failed
  yellow = in-progress / warning
  blue   = info

Config: ~/.helmix/config.yaml stores api_url and jwt_token

Build: Makefile target 'make cli-build' produces:
  dist/helmix-linux-amd64
  dist/helmix-darwin-arm64
  dist/helmix-windows-amd64.exe

Binary size target: < 15MB. Startup time: < 100ms.
```

---

## 12. Dashboard — All Pages

> Build these pages incrementally across phases. All pages are protected routes. Use shadcn/ui components throughout.

| Page / Route | Phase | Key Components |
|---|---|---|
| `/login` | Phase 1 | GitHub OAuth button, Helmix logo, tagline |
| `/dashboard` | Phase 1 | Repo list, Connect repo modal, stack badges, empty state |
| `/projects/[id]` | Phase 1 | Project overview, repo details, detected stack card |
| `/projects/[id]/infrastructure` | Phase 2 | Generated K8s manifests with syntax highlighting |
| `/projects/[id]/pipelines` | Phase 2 | Pipeline runs, stage-by-stage status, logs links |
| `/projects/[id]/deployments` | Phase 2 | Deployment history, blue/green indicator, Rollback button |
| `/projects/[id]/observability` | Phase 3 | CPU/memory/error rate/latency charts (Recharts, 24h) |
| `/projects/[id]/incidents` | Phase 3 | Incident list, AI diagnosis drawer, confidence score |
| `/projects/[id]/cost` | Phase 4 | Cost breakdown, 30-day trend chart, Optimize button |
| `/settings` | Phase 4 | Org settings, members/roles, GitHub integration, notifications |

### Real-time Features

- **WebSocket** connection to api-gateway for live deployment status updates
- **Server-Sent Events (SSE)** for live log streaming on `/projects/[id]/logs`
- **React Query** polling every 30s for metrics charts
- **WebSocket reconnect** with exponential backoff: 1s → 2s → 4s → max 30s

---

## 13. Full Testing Strategy

| Test Type | Tools | Where to Run |
|---|---|---|
| Unit tests (Go) | testing + testify + mockery | Every service, on every commit |
| Unit tests (Python) | pytest + respx (HTTP mocking) | incident-ai, on every commit |
| Integration tests | testcontainers-go (real PG/Redis) | Auth service, repo analyzer |
| K8s integration | envtest (controller-runtime) | deployment-engine |
| API contract tests | httptest + OpenAPI validation | api-gateway, all services |
| E2E tests | Playwright + Go integration | Full stack, before every release |
| Load tests | k6 | Before production deploy (1000 concurrent users) |
| LLM evaluation | Custom eval harness (20 golden cases) | Before every incident-ai change |
| Security scans | Trivy + Semgrep + Checkov | On every PR in CI |
| Chaos tests | Manual fault injection | Monthly on staging |

### 13.1 LLM Evaluation Framework

The incident AI must pass a quality bar before any change ships.

```python
# tests/eval/incident_eval.py
# Load 20 golden test cases from tests/eval/golden_cases.json
# Each case: {logs, metrics, deployment_events, expected_root_cause, expected_action}
# Run each through incident-ai with real Claude API
# Use Claude as a judge to compare root_cause to expected
# Pass criteria: >= 17/20 correct (85% accuracy)
# Output: tests/eval/report.json with per-case results + latency
```

---

## 14. CLI Reference

```bash
# First-time setup
helmix login
# → opens browser for GitHub OAuth device flow
# → stores JWT in ~/.helmix/config.yaml

# Connect a repository
helmix repos connect https://github.com/myorg/myapp
# → triggers stack detection
# → generates infrastructure
# → opens PR with CI/CD pipeline

# Deploy
helmix deploy --env staging --watch
# → triggers blue-green deployment
# → streams status: building → deploying → live

# Stream live logs
helmix logs --project my-project --tail 200 --since 30m

# View metrics
helmix metrics --project my-project --watch
# → refreshes every 5s: CPU / memory / error rate / latency

# Manage incidents
helmix incidents list --open
helmix incidents show inc_abc123
helmix incidents heal inc_abc123 --dry-run
# --dry-run: shows recommended actions without executing

# Manage secrets
helmix secrets set DATABASE_URL=postgres://...
helmix secrets list

# Rollback
helmix rollback --deployment dep_xyz456

# Cost
helmix cost --project my-project
# → shows: compute / storage / network / database monthly estimate

# Shell completion
helmix completion bash >> ~/.bashrc
helmix completion zsh  >> ~/.zshrc
```

---

## 15. Local Development Quick Start

```bash
# 1. Clone and configure
git clone https://github.com/your-org/helmix
cd helmix
cp .env.example .env
# Fill in: GITHUB_CLIENT_ID, GITHUB_CLIENT_SECRET, ANTHROPIC_API_KEY

# 2. Create local Kubernetes cluster
k3d cluster create helmix --config infra/k3d-config.yaml

# 3. Start all services
make dev
# This starts: postgres, redis, nats, qdrant, vault,
# and all Helmix microservices via docker-compose

# 4. Run database migrations
make migrate

# 5. Open dashboard
open http://localhost:3000

# 6. Run all unit tests
make test

# 7. Run end-to-end tests
make test-e2e

# 8. Stream logs for a specific service
make logs service=api-gateway
make logs service=incident-ai

# 9. Install CLI locally
make cli-build
sudo cp dist/helmix-$(uname -s | tr A-Z a-z)-$(uname -m) /usr/local/bin/helmix
helmix login
```

---

## 16. Production Deployment

### 16.1 Kubernetes Node Groups

| Node Group | Instance Type | Min / Max | Purpose |
|---|---|---|---|
| system | t3.medium (AWS) | 2 / 2 | kube-system, ingress, cert-manager, monitoring |
| services | c6i.xlarge (AWS) | 3 / 10 | All Helmix microservices (autoscaled) |
| ai | c6i.2xlarge (AWS) | 1 / 4 | incident-ai only (LLM calls are CPU-intensive) |

### 16.2 Autoscaling Targets

| Service | Min | Max | Scale Trigger |
|---|---|---|---|
| api-gateway | 2 | 20 | CPU > 70% |
| repo-analyzer | 2 | 10 | NATS queue depth > 50 |
| infra-generator | 2 | 8 | NATS queue depth > 20 |
| pipeline-generator | 2 | 6 | NATS queue depth > 20 |
| deployment-engine | 2 | 6 | CPU > 60% |
| incident-ai | 1 | 4 | NATS queue depth > 10 |
| observability | 2 | 4 | Memory > 80% |

### 16.3 Helmix's Own CI/CD Pipeline

Helmix deploys itself. The platform's CI/CD (`.github/workflows/platform-deploy.yml`):

| Stage | What Happens |
|---|---|
| lint | golangci-lint (Go) + ruff + mypy (Python) + eslint (Next.js) |
| test | Unit + integration tests for all services in parallel |
| eval | LLM evaluation harness — fail build if < 85% accuracy on 20 golden cases |
| security | Trivy scan + Semgrep SAST + Checkov IaC |
| build | Docker build all services with layer caching |
| push | Push to GHCR — `:sha` and `:latest` on main branch |
| deploy-staging | `helmix deploy helmix-platform --env=staging` |
| smoke-test | Playwright E2E against staging |
| deploy-prod | `helmix deploy --env=prod` (manual approval gate required) |

---

## 17. Coding Standards

### Go Services

- Every service must have a `/health` endpoint: `{status:'ok', service:'name', version:'0.1.0'}`
- All errors wrapped with context: `fmt.Errorf('creating user: %w', err)`
- All database queries use prepared statements — no string interpolation
- All environment variables validated at startup via a `Config` struct
- Log format: structured JSON using `slog` with fields: `service`, `trace_id`, `level`, `msg`
- Every public function must have a Go doc comment
- Use `context.Context` for all operations with timeouts
- All timeouts configurable via environment variables
- Blue-green state and all deployment state in PostgreSQL, never in memory

### Python Services

- Use Pydantic v2 for all data models — no raw dicts passed between functions
- Use `structlog` for structured logging
- LLM calls have 30s hard timeout and 3-retry exponential backoff
- All LLM interactions logged (prompt + response) for audit
- Auto-remediation actions must be idempotent
- Qdrant collections created on startup if missing
- Auto-heal disabled by default per project — user must opt in

### General Rules

- Never start Phase N+1 until all Phase N acceptance tests pass
- Services communicate only through NATS events — never direct HTTP between services
- Event schemas are the contract — do not change them without versioning
- Never put real secrets in prompts — use placeholder env var names
- Every NATS event consumed and emitted must be logged with `project_id` + `trace_id`
- Terraform plan always requires human approval before apply, even in automation

---

## 18. Vibe Coding Golden Rules

| # | Rule |
|---|---|
| 1 | **NEVER skip tests.** Every phase has acceptance criteria. Run them before moving on. |
| 2 | **ONE SERVICE AT A TIME.** Build, test, containerize each service before connecting it. |
| 3 | **PASTE FULL PROMPTS.** Always paste the entire prompt — context is critical for AI. |
| 4 | **VERTICAL SLICES.** Each prompt produces a working feature, not a horizontal layer. |
| 5 | **NATS IS THE CONTRACT.** No direct HTTP calls between services (except api-gateway). |
| 6 | **LLM CALLS ARE SLOW.** Always run Claude API in background workers, never in hot path. |
| 7 | **INFRA NEEDS REVIEW.** Terraform plan always requires human approval before apply. |
| 8 | **LOG EVERYTHING.** Every event consumed/emitted must be logged with `project_id` + `trace_id`. |
| 9 | **EMBED CONTEXT, NOT CREDENTIALS.** Never put real secrets in prompts. |
| 10 | **PHASE 0 FIRST, ALWAYS.** If docker-compose doesn't work, nothing else will. |
| 11 | **DB STATE ONLY.** All state (blue-green, deployment status) lives in PostgreSQL. |
| 12 | **CONFIDENCE THRESHOLD.** AI auto-heals ONLY when confidence >= 0.85. Below that: notify. |

---

*⬡ Helmix — Connect your repo. Everything else is automatic.*

[github.com/your-org/helmix](https://github.com/your-org/helmix)
