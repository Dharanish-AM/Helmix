# Helmix Project Tracker

Last updated: 2026-03-13
Owner: Core Team
Source plan: Helmix_Full_Implementation_Guide.md

## Permanent Reference Rule

This tracker is permanently bound to Helmix_Full_Implementation_Guide.md.

Mandatory update rules:
- Every task update in this file must map to a phase or section in Helmix_Full_Implementation_Guide.md.
- Do not add scope items that are not present in Helmix_Full_Implementation_Guide.md unless explicitly marked as Out of Scope Proposal.
- If the guide changes, update this tracker in the same session.
- Keep section names consistent with the guide to avoid planning drift.

## Guide Section Mapping

| Tracker Area | Guide Section Reference |
|---|---|
| Global Progress Dashboard | Section 6 Phase-wise Build Roadmap |
| Phase 0 Checklist | Section 7 Phase 0 Foundation |
| Phase 1 Checklist | Section 8 Phase 1 GitHub Integration and Foundation Services |
| Phase 2 Checklist | Section 9 Phase 2 Infrastructure Generator, Pipelines and Deployment |
| Phase 3 Checklist | Section 10 Phase 3 Observability and AI Incident Engine |
| Phase 4 Checklist | Section 11 Phase 4 Production Hardening |
| Acceptance Criteria Tracker | End of each phase acceptance criteria blocks |

## How to Use This File

Update this file at the end of each work session.

Status values:
- Not Started
- In Progress
- Blocked
- Done

Priority values:
- P0 (critical)
- P1 (high)
- P2 (medium)
- P3 (low)

## Global Progress Dashboard

| Phase | Name | Target Window | Status | Completion | Notes |
|---|---|---|---|---|---|
| 0 | Foundation | Days 1-3 | Done | 100% | All 7 Phase 0 acceptance criteria verified Pass |
| 1 | GitHub Integration and Foundation Services | Weeks 1-3 | Done | 100% | Auth service, shared JWT middleware, API gateway, repo-analyzer, dashboard auth flow, integration tests, and Phase 1 e2e validation completed |
| 2 | Infra Generator, Pipelines and Deployment | Weeks 4-6 | Done | 100% | Infra-generator, pipeline-generator, and deployment-engine are implemented, gateway-integrated, compose-smoke verified, and the full analyze->infra->pipeline->deploy->rollback flow passes |
| 3 | Observability and AI Incident Engine | Weeks 7-10 | Not Started | 0% | Pending Phase 2 completion |
| 4 | Production Hardening | Weeks 11-12 | Not Started | 0% | Pending Phase 3 completion |

## Active Sprint Focus

Current sprint goal: Complete Phase 2 deployment-engine slice and close Phase 2 acceptance verification.

| Task ID | Task | Priority | Owner | Status | Due Date | Updated |
|---|---|---|---|---|---|---|
| P2-01 | Implement infra-generator `/generate` endpoint with stack template selection | P0 | Team | Done | 2026-03-16 | 2026-03-13 |
| P2-02 | Add infra-generator unit and API tests for supported and unsupported stacks | P0 | Team | Done | 2026-03-16 | 2026-03-13 |
| P2-03 | Add Phase 2 acceptance criteria table and command evidence fields | P1 | Team | Done | 2026-03-17 | 2026-03-13 |
| P2-04 | Integrate infra-generator into gateway and compose runtime path | P1 | Team | Done | 2026-03-17 | 2026-03-13 |
| P2-05 | Implement pipeline-generator `/generate` endpoint with workflow template selection | P0 | Team | Done | 2026-03-17 | 2026-03-13 |
| P2-06 | Add pipeline-generator unit and API tests plus compose wiring | P0 | Team | Done | 2026-03-17 | 2026-03-13 |
| P2-07 | Implement deployment-engine `/deploy`, status, and rollback endpoints with DB-backed state transitions | P0 | Team | Done | 2026-03-18 | 2026-03-13 |
| P2-08 | Add deployment-engine proxy, compose smoke, and full flow e2e coverage | P0 | Team | Done | 2026-03-18 | 2026-03-13 |

## Phase Checklists

### Phase 0 Checklist

- [x] Monorepo folder structure created
- [x] Root go.work created
- [x] Root Makefile created
- [x] turbo.json pipeline created
- [x] .env.example added
- [x] Core infra services in docker-compose with health checks
- [x] SQL migrations created with up and down files
- [x] Required indexes added in migrations
- [x] libs event-sdk types added
- [x] libs event-sdk publish and subscribe helpers added
- [x] libs event-sdk unit tests added
- [x] make dev verified end-to-end on local machine
- [x] make migrate verified against fresh postgres
- [x] All Phase 0 acceptance checks marked passed

### Phase 1 Checklist

- [x] Auth service endpoints complete
- [x] JWT middleware and role middleware in libs auth
- [x] API gateway middleware stack complete
- [x] Repo analyzer detection logic complete
- [x] Dashboard auth flow complete
- [x] Phase 1 unit, integration, e2e tests passing

### Phase 2 Checklist

- [x] Infra generator templates and validation complete
- [x] Pipeline generator workflow and PR creation complete
- [x] Deployment engine blue-green and rollback complete
- [x] Phase 2 unit and e2e tests passing

### Phase 3 Checklist

- [ ] Observability metric pipeline and alerting rules complete
- [ ] Incident AI diagnosis and context gathering complete
- [ ] Auto-remediation workflow complete
- [ ] Qdrant memory integration complete
- [ ] Phase 3 unit and e2e tests passing

### Phase 4 Checklist

- [ ] Multi-tenancy and RBAC complete
- [ ] Vault secret management complete
- [ ] Terraform modules for cloud providers complete
- [ ] Security hardening controls complete
- [ ] Full CLI command set complete
- [ ] Production readiness checklist passed

## Blockers and Risks Log

| ID | Date | Type | Description | Impact | Owner | Mitigation | Status |
|---|---|---|---|---|---|---|---|
| R-001 | 2026-03-13 | Environment | Go toolchain availability was initially unclear in the execution environment | Medium | Team | Validated `go test` locally against libs/auth, auth-service, and api-gateway modules | Mitigated |
| R-002 | 2026-03-13 | Environment | make dev used compose watch mode but no services are configured for watch, causing early failure | High | Team | Updated Makefile dev target to fall back to docker compose up -d | Mitigated |
| R-003 | 2026-03-13 | Tooling | Host-local migration execution introduced nondeterministic failures (driver/toolchain mismatch) | High | Team | Switched Makefile migrate targets to run pinned `migrate/migrate` image in Docker network context only | Mitigated |
| R-004 | 2026-03-13 | Runtime | Migrate runs before Postgres role initialization is fully ready, causing role helmix does not exist errors | High | Team | Added wait-postgres target and wired migrate to wait for successful helmix login | Mitigated |
| R-005 | 2026-03-13 | Runtime | Postgres role/database bootstrap remains inconsistent in local startup edge cases | High | Team | Added ensure-postgres-bootstrap target to auto-create helmix role/database before migrations | Mitigated |

## Decisions Log

| Date | Decision | Reason | Owner |
|---|---|---|---|
| 2026-03-13 | Use PROJECT_TRACKER.md as persistent operational tracker | Single source of execution truth in repository | Team |

## Session Update Log

Use one line per update.

| Date | Update |
|---|---|
| 2026-03-13 | Phase 0 scaffold created, migrations added, event-sdk implemented with tests, dashboard scaffold created |
| 2026-03-13 | Added permanent guide-reference policy and explicit section mapping to keep tracker aligned with Helmix_Full_Implementation_Guide.md |
| 2026-03-13 | Converted Phase 0 acceptance into strict Pass/Fail verification sheet with command and evidence fields |
| 2026-03-13 | Replaced .github instructions template with Helmix-specific coding, architecture, phase gating, and tracker update rules |
| 2026-03-13 | Executed Phase 0 commands: event-sdk tests passed, but Docker daemon unavailable blocked infrastructure and migration verification |
| 2026-03-13 | Re-ran Phase 0 commands: Docker daemon available, new blockers identified and mitigated in Makefile (watch fallback and postgres migrate tag) |
| 2026-03-13 | Re-ran validation: stack starts in detached fallback mode, qdrant ready, but migrations blocked by Postgres role mismatch (role helmix missing) |
| 2026-03-13 | Added make wait-postgres gate so migrate waits for successful helmix role login before applying migrations |
| 2026-03-13 | Added ensure-postgres-bootstrap gate to auto-create helmix role/database before migration attempts |
| 2026-03-13 | Switched `make migrate` and `make migrate-down` to deterministic Docker-only execution using pinned migrate image and container-network DB URL; verified `make migrate` succeeds (`no change`) |
| 2026-03-13 | Validated Docker-context migration cycle by running `make migrate-down` then `make migrate`; schema verified via `docker exec helmix-postgres psql` |
| 2026-03-13 | Fixed docker-compose health checks: NATS (PID file + nats.conf + reload signal), Qdrant (bash /dev/tcp), Vault (wget 127.0.0.1 no --spider); all 5 containers reach healthy state |
| 2026-03-13 | Phase 0 complete — all 7 acceptance criteria pass |
| 2026-03-13 | Phase 1 started: implemented auth-service OAuth/JWT/refresh flow, shared libs/auth middleware, auth schema migration, and api-gateway request pipeline; verified with `go test ./...` in libs/auth, services/auth-service, and services/api-gateway |
| 2026-03-13 | Applied migration `000007_add_auth_columns` successfully via `make migrate` to add encrypted GitHub token storage columns to users |
| 2026-03-13 | Implemented repo-analyzer stack detection service with persistence and `repo.analyzed` event publication; verified with `go test ./...` in services/repo-analyzer |
| 2026-03-13 | Completed dashboard auth/session flow and verified frontend production build with `NODE_ENV=production npm run build` after removing client `useSearchParams` from prerender path |
| 2026-03-13 | Added auth-service and api-gateway integration tests plus Phase 1 e2e test (`tests/e2e/phase1_test.go`); validated e2e pass in compose network via `docker run --network helmix_default ... go test . -run TestPhase1AnalyzeViaGatewayPublishesEvent -v` |
| 2026-03-13 | Started Phase 2 by implementing infra-generator config/server modules and `/generate` endpoint with stack-based Docker template generation for Next.js and FastAPI; verified with `cd services/infra-generator && go test ./...` |
| 2026-03-13 | Extended Phase 2 infra-generator with API-level server tests, added `infra-generator` service to `docker-compose`, added `services/infra-generator` to CI go-test matrix, and validated runtime generation via `curl -X POST http://localhost:8083/generate ...` |
| 2026-03-13 | Added authenticated api-gateway integration test for `/api/v1/infra/generate` proxy path (method/path/body forwarding to infra-generator upstream) and verified with `cd services/api-gateway && go test ./...` |
| 2026-03-13 | Added Phase 2 acceptance criteria tracker and validated initial gates: runtime generation (`curl http://localhost:8083/generate`), infra-generator/api-gateway tests (`go test ./...`), and CI matrix coverage for `services/infra-generator` |
| 2026-03-13 | Added compose-level Phase 2 smoke target `make test-e2e-phase2-infra` with authenticated JWT request through api-gateway to infra-generator and validated pass in Docker network |
| 2026-03-13 | Implemented pipeline-generator config/server modules and `/generate` endpoint with GitHub Actions workflow templates for Next.js and FastAPI, added unit/API tests, wired service into `docker-compose` and CI matrix, and validated runtime generation via `curl -X POST http://localhost:8084/generate ...` |
| 2026-03-13 | Added authenticated api-gateway integration test for `/api/v1/pipelines/generate` plus compose-level smoke target `make test-e2e-phase2-pipeline`; validated both with `cd services/api-gateway && go test ./...` and Docker-network e2e pass |
| 2026-03-13 | Added chained Phase 2 e2e target `make test-e2e-phase2-flow` to validate analyze -> infra -> pipeline generation through api-gateway using a local temporary Git repo fixture; validated pass in Docker network |
| 2026-03-13 | Implemented deployment-engine with DB-backed deploy/status/rollback endpoints, added api-gateway proxy coverage, wired compose and CI, and validated `make test-e2e-phase2-deploy` plus full `make test-e2e-phase2-flow` analyze -> infra -> pipeline -> deploy -> rollback flow |

## Acceptance Criteria Tracker

### Phase 0 Acceptance

Use this section as the single verification sheet. Replace Pending with Pass or Fail and paste brief command evidence.

| ID | Criterion | Status (Pass/Fail/Pending) | Validation Command | Evidence |
|---|---|---|---|---|
| AC-0-01 | make dev starts all containers with no errors | Pass | make dev | watch unsupported message shown, fallback started containers successfully with docker compose up -d |
| AC-0-02 | All container health checks pass | Pass | docker ps | all 5 containers healthy: postgres (healthy), redis (healthy), nats (healthy), qdrant (healthy), vault (healthy) |
| AC-0-03 | make migrate succeeds on fresh database | Pass | make migrate | migrations run via Docker migrate image in container network context; command result: no change |
| AC-0-04 | All 8 tables exist with expected columns | Pass | docker exec helmix-postgres psql -U helmix -d helmix -Atc "SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename;" | verified schema tables in container context: alerts, deployments, incidents, infra_resources, org_members, organizations, pipelines, projects, repos, users (plus schema_migrations metadata table) |
| AC-0-05 | NATS web UI is accessible | Pass | curl -sf http://localhost:8222/healthz | response: {"status":"ok"} |
| AC-0-06 | Qdrant API is accessible | Pass | curl -sS http://localhost:6333/readyz | response: all shards are ready |
| AC-0-07 | libs event-sdk tests pass | Pass | cd libs/event-sdk && go test ./... | ok github.com/your-org/helmix/libs/event-sdk 0.644s |

#### Phase 0 Verification Snapshot

| Field | Value |
|---|---|
| Verified by | dharanisham |
| Verification date | 2026-03-13 |
| Environment | Local macOS |
| Result | Pass |
| Notes | All 7 Phase 0 acceptance criteria verified pass. All containers healthy, migrations clean, schema confirmed, event-sdk tests pass. |

### Phase 2 Acceptance

Use this section as the verification sheet for early Phase 2 increments. Replace Pending with Pass or Fail and paste brief command evidence.

| ID | Criterion | Status (Pass/Fail/Pending) | Validation Command | Evidence |
|---|---|---|---|---|
| AC-2-01 | infra-generator `/generate` returns template output for supported stack | Pass | curl -sS -X POST http://localhost:8083/generate -H 'Content-Type: application/json' -d '{"project_slug":"demo-next","provider":"docker","stack":{"runtime":"node","framework":"nextjs"}}' | response includes `"template":"docker-nextjs"` and generated file payload |
| AC-2-02 | infra-generator module tests pass (generator + server) | Pass | cd services/infra-generator && go test ./... | tests pass: `internal/generator` and `internal/server` |
| AC-2-03 | api-gateway has authenticated proxy coverage for `/api/v1/infra/generate` | Pass | cd services/api-gateway && go test ./... | `internal/gateway` test suite passes with authenticated infra proxy integration test |
| AC-2-04 | CI includes infra-generator in Go test matrix | Pass | grep -n "services/infra-generator" .github/workflows/ci.yml | `.github/workflows/ci.yml` includes matrix entry at line 58 |
| AC-2-05 | Compose-level authenticated smoke flow for gateway -> infra-generator passes | Pass | make test-e2e-phase2-infra | `TestPhase2GatewayInfraGenerateAuthorized` passes against `http://api-gateway:8080/api/v1/infra/generate` in Docker network |
| AC-2-06 | pipeline-generator workflow generation implemented and validated | Pass | cd services/pipeline-generator && go test ./... && curl -sS -X POST http://localhost:8084/generate -H 'Content-Type: application/json' -d '{"project_slug":"demo-next","provider":"github-actions","stack":{"runtime":"node","framework":"nextjs"}}' | tests pass and runtime response includes `"template":"github-actions-nextjs"` |
| AC-2-07 | api-gateway has authenticated proxy coverage for `/api/v1/pipelines/generate` | Pass | cd services/api-gateway && go test ./... | `internal/gateway` test suite passes with authenticated pipeline proxy integration test |
| AC-2-08 | Compose-level authenticated smoke flow for gateway -> pipeline-generator passes | Pass | make test-e2e-phase2-pipeline | `TestPhase2GatewayPipelineGenerateAuthorized` passes against `http://api-gateway:8080/api/v1/pipelines/generate` in Docker network |
| AC-2-09 | deployment-engine blue-green/rollback path implemented and validated | Pass | make test-e2e-phase2-deploy | `TestPhase2GatewayDeploymentRollbackAuthorized` passes against `http://api-gateway:8080/api/v1/deployments/*` with live transition and rollback restoring previous deployment |
| AC-2-10 | Phase 2 chained flow test passes (analyze -> infra -> pipeline) | Pass | make test-e2e-phase2-flow | Full flow validation includes successful analyze -> infra -> pipeline generation through api-gateway before deployment starts |
| AC-2-11 | Full deployment path (analyze -> infra -> pipeline -> deploy) passes | Pass | make test-e2e-phase2-flow | `TestPhase2AnalyzeInfraPipelineDeployFlow` passes end-to-end through api-gateway including deploy promotion and rollback |

#### Phase 2 Verification Snapshot

| Field | Value |
|---|---|
| Verified by | dharanisham |
| Verification date | 2026-03-13 |
| Environment | Local macOS |
| Result | Pass |
| Notes | Full analyze -> infra -> pipeline -> deploy -> rollback flow now passes end-to-end through the gateway; deployment-engine module, proxy, compose smoke, and CI matrix coverage are in place. |

## Next Actions

1. Start Phase 3 observability service slice and define the first alert ingestion/status endpoints.
2. Decide whether Phase 2 compose smoke targets should become mandatory CI jobs in addition to module tests.
3. Add dashboard deployment history and rollback UI once the Phase 3 backend work is underway.
