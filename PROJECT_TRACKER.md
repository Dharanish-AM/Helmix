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
| 3 | Observability and AI Incident Engine | Weeks 7-10 | In Progress | 80% | Observability latency and zero-pod rule tests expanded; incident-ai deployment context enriched with structured DeploymentContext, concrete per-rule action routing, and 14 Python tests; dashboard incidents and observability pages live |
| 4 | Production Hardening | Weeks 11-12 | Not Started | 0% | Pending Phase 3 completion |

## Active Sprint Focus

Current sprint goal: Validate the first complete Phase 3 backend loop: observability alert -> incident-ai diagnosis -> manual action event flow.

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
| P3-01 | Implement observability snapshot ingestion, recent metrics API, and open alerts API | P0 | Team | Done | 2026-03-20 | 2026-03-13 |
| P3-02 | Add observability rule evaluation, alert deduplication, and alert.fired publication | P0 | Team | Done | 2026-03-20 | 2026-03-13 |
| P3-03 | Implement incident-ai alert intake, diagnosis persistence, and incident query endpoints | P0 | Team | Done | 2026-03-21 | 2026-03-13 |
| P3-04 | Add manual incident action path and incident-created smoke validation | P0 | Team | Done | 2026-03-21 | 2026-03-13 |
| P3-05 | Expand observability latency and zero-pod rule unit and e2e smoke coverage | P0 | Team | Done | 2026-03-22 | 2026-03-13 |
| P3-06 | Enrich incident-ai deployment history context (DeploymentContext model, minutes_since_deploy) | P0 | Team | Done | 2026-03-22 | 2026-03-13 |
| P3-07 | Add per-rule concrete manual action routing and restart-without-deployment-id fallback | P0 | Team | Done | 2026-03-22 | 2026-03-13 |
| P3-08 | Add dashboard /incidents and /observability UI pages and Phase 3 API fetchers in lib/api.ts | P1 | Team | Done | 2026-03-22 | 2026-03-13 |

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
| 2026-03-13 | Started Phase 3 observability slice: added metric snapshot migration, observability snapshot ingestion and alert APIs, alert rule evaluation + deduplication + `alert.fired` NATS publication, gateway proxy coverage, and Phase 3 smoke target wiring |
| 2026-03-13 | Validated Phase 3 observability foundation with `cd services/observability && go test ./...`, `cd services/api-gateway && go test ./...`, and `make test-e2e-phase3-observability`; synthetic error-rate snapshots now produce an open alert and `alert.fired` event through api-gateway |
| 2026-03-13 | Started incident-ai slice: added FastAPI service scaffold, alert.fired subscription, deterministic diagnosis provider abstraction, incident persistence, manual action API, Python unit tests, compose wiring, and Phase 3 incident smoke target |
| 2026-03-13 | Validated incident-ai foundation with containerized `pytest tests -q`, `cd services/api-gateway && go test ./...`, and `make test-e2e-phase3-incident`; alert.fired now produces incident.created and manual incident actions publish autoheal.triggered |
| 2026-03-13 | Expanded observability alerting rules_test.go with 9 new tests covering p99-latency-high (fires/no-fire/gap/exact-threshold/severity) and ready-pods-zero (severity/partial-pods/immediate/value+threshold); all 14 tests pass |
| 2026-03-13 | Added two new e2e smoke test functions: TestPhase3ObservabilityLatencyAlert (6 high-latency snapshots → alert.fired metric=p99_latency_ms severity=warning) and TestPhase3ObservabilityZeroPodAlert (1 zero-ready-pod snapshot → alert.fired metric=ready_pod_count severity=critical) |
| 2026-03-13 | Enriched incident-ai context_clients.py with DeploymentContext pydantic model and _parse_deployment parsing started_at into minutes_since_deploy; service._build_prompt now emits structured deployment-history section with recent-deploy warning annotation |
| 2026-03-13 | Added _RULE_ACTION_SEQUENCES map and recommended_action_sequence() to IncidentService; _execute_action now handles restart_pods/rollback_deployment/scale_pods gracefully without deployment_id (returns accepted); MockProvider now routes by rule/metric keyword |
| 2026-03-13 | Extended Python test suite to 14 tests: test_deployment_context_in_prompt, test_latency_alert_routing, test_zero_pod_alert_routing, test_recommended_action_sequence_per_rule, test_restart_pods_without_deployment_id_accepted, test_rollback_without_deployment_id_accepted, test_scale_pods_without_deployment_id_accepted; all 14 pass |
| 2026-03-13 | Created frontend/dashboard/app/dashboard/incidents/page.tsx and app/dashboard/observability/page.tsx; added fetchCurrentMetrics, fetchAlerts, fetchIncidents, triggerIncidentAction to lib/api.ts; production build passes (8/8 static pages) |
| 2026-03-13 | Aligned dashboard incidents API client with gateway route (`/api/v1/incidents/projects/{project_id}`) and expanded `make test-e2e-phase3-observability` to run all `TestPhase3Observability*` smoke tests (alert flow + latency + zero-pod) |
| 2026-03-13 | Added quick navigation from dashboard shell to `/dashboard/observability` and `/dashboard/incidents` and cleaned minor incident-ai provider formatting issue; production build remains green |
| 2026-03-13 | Added deployment-aware incident action controls in dashboard incidents UI: per-incident Deployment ID input is now included in rollback/scale/restart params when provided; production build remains green |
| 2026-03-13 | Implemented deployment history list endpoint in deployment-engine (`GET /deployments?project_id=&limit=`), added api-gateway proxy test coverage, and upgraded dashboard incidents action control from manual deployment-id input to live deployment picker; validated with `go test` in deployment-engine/api-gateway and production dashboard build |
| 2026-03-13 | Added explicit Phase 3 gateway route-level e2e coverage for dashboard incident paths: `GET /api/v1/incidents/projects/{project_id}` and `POST /api/v1/incidents/{incident_id}/actions`; broadened `make test-e2e-phase3-incident` to run all `TestPhase3Incident*` tests |
| 2026-03-13 | Added manual refresh and configurable auto-refresh controls (10s/15s/30s/60s) to dashboard incidents and observability pages with last-refreshed timestamp; production dashboard build and TS diagnostics pass |
| 2026-03-13 | Added deployment picker filtering controls in dashboard incidents (environment + status) with filtered-count visibility; validated production dashboard build and diagnostics |
| 2026-03-13 | Added incident detail panel in dashboard incidents with on-demand detail/similar fetch (`/api/v1/incidents/{id}` + `/api/v1/incidents/{id}/similar`) and added per-incident last manual action summary card; production dashboard build and diagnostics pass |
| 2026-03-13 | Added compact action-status badges (accepted/failed/running) and incidents pagination controls (page size + prev/next + page indicator) to improve live-incident scanning and scale; production dashboard build and diagnostics pass |
| 2026-03-13 | Added observability->incidents quick-link flow: open-alert CTA + table Investigate action now route to `/dashboard/incidents` with query context (`project_id`, `alert_id`, `alert_rule`, `alert_metric`); incidents page pre-fills project, auto-loads data, and shows triage banner; production dashboard build passes |
| 2026-03-13 | Implemented server-side incidents pagination contract in incident-ai (`limit`/`offset` + `items/total/limit/offset` response envelope), updated dashboard incidents fetcher/UI to consume backend pagination, and validated with `pytest tests -q`, focused e2e `go test -run TestPhase3IncidentGatewayRoutes`, and production dashboard build |
| 2026-03-13 | Added deep-link action presets in dashboard incidents: observability alert rule context now preselects per-incident manual action (rule->action map), renders suggested preset banner, and supports one-click `Run Selected`; production dashboard build and diagnostics pass |
| 2026-03-13 | Added incident-ai pagination regression coverage (`test_list_incidents_returns_paginated_response_contract`) validating repository passthrough and `items/total/limit/offset` response metadata; Python suite now 15 passing tests |
| 2026-03-13 | Added lightweight frontend smoke test for incidents preset-selection behavior (`suggestedActionForAlertRule`) with Vitest, extracted preset mapping helper module, and validated with `npm run test` plus production dashboard build |
| 2026-03-13 | Removed transitional legacy-array incidents decoding from route-level Phase 3 e2e, tightened pagination-envelope assertions (`items/total/limit/offset` with explicit `limit=5&offset=0`), and added dedicated incident deep-link rule parsing (`alert_rule` with `rule` alias) plus Vitest coverage |
| 2026-03-13 | Revalidated strict incidents pagination contract end-to-end via `make test-e2e-phase3-incident` after compose force-recreate; both `TestPhase3IncidentFlow` and `TestPhase3IncidentGatewayRoutes` pass with required `items/total/limit/offset` envelope |
| 2026-03-13 | Hardened migration startup path for CI/local by starting Postgres before migrate in both Makefile and Phase 1 CI job; validated with local `make migrate` and `make test-e2e-phase1` pass |

### Phase 3 Acceptance

Use this section as the verification sheet for the initial Phase 3 slice. Replace Pending with Pass or Fail and paste brief command evidence.

| ID | Criterion | Status (Pass/Fail/Pending) | Validation Command | Evidence |
|---|---|---|---|---|
| AC-3-01 | observability snapshot ingestion persists metrics and exposes current snapshot API | Pass | cd services/observability && go test ./... && make test-e2e-phase3-observability | module tests pass and smoke test reads latest metrics via `/api/v1/observability/metrics/{project_id}/current` |
| AC-3-02 | alert rules fire and deduplicate open alerts for sustained breaches | Pass | cd services/observability && go test ./... && make test-e2e-phase3-observability | rules tests pass and smoke test injects 4 high error-rate snapshots, producing one open critical alert |
| AC-3-03 | observability publishes `alert.fired` and is reachable via gateway proxy | Pass | cd services/api-gateway && go test ./... && make test-e2e-phase3-observability | gateway proxy tests pass and smoke test receives `alert.fired` on NATS after posting snapshots through api-gateway |
| AC-3-04 | incident-ai subscribes to `alert.fired`, persists diagnosis, and publishes `incident.created` | Pass | docker run --rm -v "$PWD:/workspace" -w /workspace/services/incident-ai python:3.12-slim sh -c "pip install --no-cache-dir -r requirements.txt >/tmp/pip.log && pytest tests -q" && make test-e2e-phase3-incident | Python tests pass and smoke test confirms `incident.created` after synthetic alert flow |
| AC-3-05 | manual incident action path publishes `autoheal.triggered` through gateway | Pass | cd services/api-gateway && go test ./... && make test-e2e-phase3-incident | gateway proxy tests pass and smoke test POST to `/api/v1/incidents/{id}/actions` emits `autoheal.triggered` |
| AC-3-06 | p99-latency-high rule fires after 6 consecutive high-latency snapshots and not before | Pass | cd services/observability && go test ./internal/alerting/... && make test-e2e-phase3-observability | 14 unit tests pass (TestP99LatencyHighFires, TestP99LatencyHighNoFireFewSnapshots, TestP99LatencyHighNoFireGap, TestP99LatencyHighExactThreshold, TestP99LatencySeverityIsWarning); e2e TestPhase3ObservabilityLatencyAlert passes |
| AC-3-07 | ready-pods-zero rule fires immediately on a single snapshot with no ready pods | Pass | cd services/observability && go test ./internal/alerting/... && make test-e2e-phase3-observability | 14 unit tests pass (TestZeroPodSeverityIsCritical, TestZeroPodPartialPodsHealthy, TestZeroPodFiresImmediatelyWithoutHistory, TestZeroPodValueAndThresholdInAlert); e2e TestPhase3ObservabilityZeroPodAlert passes |
| AC-3-08 | incident-ai embeds structured deployment history (deployment_id, image, minutes_since_deploy) in LLM prompt | Pass | /tmp/incident-ai-venv313/bin/pytest tests -v | test_deployment_context_in_prompt passes; dep-99 and image annotation visible in captured prompt |
| AC-3-09 | per-rule action routing: ready-pods-zero→restart_pods, p99-latency-high→scale_pods, error-rate-high→rollback_deployment | Pass | /tmp/incident-ai-venv313/bin/pytest tests -v | test_recommended_action_sequence_per_rule, test_latency_alert_routing, test_zero_pod_alert_routing all pass |
| AC-3-10 | dashboard /incidents and /observability pages build and render without errors | Pass | cd frontend/dashboard && NODE_ENV=production npm run build | 8/8 static pages generated; /dashboard/incidents and /dashboard/observability in route table |
| AC-3-11 | incident-ai dashboard routes are validated through api-gateway (`GET /api/v1/incidents/projects/{project_id}` + `POST /api/v1/incidents/{id}/actions`) | Pass | make test-e2e-phase3-incident | `TestPhase3IncidentFlow` and `TestPhase3IncidentGatewayRoutes` both pass through compose/api-gateway path |

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

1. Add an incidents deep-link smoke test that opens `/dashboard/incidents?project_id=...&alert_rule=...` and verifies suggested action preset rendering.
2. Expand gateway integration assertions for incidents list to validate response JSON shape end-to-end (not only proxy passthrough).
3. Run compose-level `make test-e2e-phase3-incident` in CI after incident-ai image refresh to enforce pagination-envelope contract across environments.
