# Helmix Project Tracker

Last updated: 2026-03-14
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
| 3 | Observability and AI Incident Engine | Weeks 7-10 | Done | 100% | All 11 Phase 3 acceptance criteria verified Pass locally and on CI; phase1-e2e-smoke and phase3-incident-e2e-smoke both passed on GitHub Actions run #5 (3m 46s, Success) |
| 4 | Production Hardening | Weeks 11-12 | In Progress | 82% | Core Phase 4 tracks are implemented, but guide-alignment gaps remain: audit_logs persistence, deploy-time image scan persistence/override, K8s security artifacts, platform deploy automation, and missing product/test surfaces |

## Active Sprint Focus

Current sprint goal: Close the verified guide-alignment gaps required for an honest 100% completion claim.

Sprint status note: Validation shows the repo is broadly working, but still missing several guide-defined deliverables across Sections 10-16. Work has resumed on the highest-priority missing slices starting with auth audit logging.

| Task ID | Task | Priority | Owner | Status | Due Date | Updated |
|---|---|---|---|---|---|---|
| P4-01 | Implement org management endpoints (create, invite, accept-invite, list/update/remove members) in auth-service | P0 | Team | Done | 2026-03-13 | 2026-03-13 |
| P4-02 | Add org_invites migration, RequireRole tests, gateway proxy for /api/v1/orgs | P0 | Team | Done | 2026-03-13 | 2026-03-13 |
| P4-03 | Implement Vault integration (AppRole per service, secrets CRUD API) | P0 | Team | Done | 2026-03-14 | 2026-03-14 |
| P4-04 | Create Terraform modules (VPC, K8s, DB, cache, registry) for AWS/GCP/Azure | P1 | Team | Done | 2026-03-15 | 2026-03-14 |
| P4-05 | Security hardening: headers middleware, input validation, Trivy scanning, rate limiting | P0 | Team | Done | 2026-03-15 | 2026-03-14 |
| P4-06 | Full CLI (Cobra + Viper): all commands + make cli-build for 3 platforms | P1 | Team | Done | 2026-03-16 | 2026-03-14 |
| GAP-01 | Add audit_logs migration and auth-service event persistence for login/logout/auth failures | P0 | Team | In Progress | 2026-03-15 | 2026-03-14 |
| GAP-02 | Add deployment scan_results persistence and accept_risk override path in deployment-engine/api-gateway | P0 | Team | Done | 2026-03-16 | 2026-03-14 |
| GAP-03 | Add notification-service and wire event consumers into compose/CI | P1 | Team | Not Started | 2026-03-17 | 2026-03-14 |
| GAP-04 | Add infra/kubernetes and infra/helm-charts/helmix deployment artifacts including security resources | P1 | Team | Not Started | 2026-03-18 | 2026-03-14 |
| GAP-05 | Add missing dashboard surfaces: logs, deployments, pipelines, cost, settings, plus live WS/SSE flows | P1 | Team | Not Started | 2026-03-19 | 2026-03-14 |
| GAP-06 | Add Playwright E2E, k6 load tests, and incident-ai eval harness | P1 | Team | Not Started | 2026-03-20 | 2026-03-14 |
| GAP-07 | Add platform-deploy workflow and CI gates for mypy, Semgrep, Checkov, staging smoke, and prod gate | P1 | Team | Not Started | 2026-03-21 | 2026-03-14 |

## Verified Guide-Alignment Gap Backlog

These items were verified against Helmix_Full_Implementation_Guide.md and the current repo contents on 2026-03-14.

| Gap ID | Guide Section | Missing Deliverable | Status | Notes |
|---|---|---|---|---|
| VG-01 | Section 11 Phase 4.4 | `audit_logs` table and auth event persistence | In Progress | Migration, auth-service persistence, route-level audit tests, and org/secret handler audit coverage are in place; remaining work is taxonomy cleanup and closeout decision |
| VG-02 | Section 11 Phase 4.4 | `deployments.scan_results` persistence and `accept_risk` override | Done | Migration `000011` applied; deployment-engine persists scan metadata and enforces risk override; gateway forwarding and e2e rejection/override paths verified |
| VG-03 | Sections 2 and 10 | `notification-service` implementation | Not Started | Referenced in architecture/event flow but absent from repo |
| VG-04 | Sections 2 and 16 | `infra/kubernetes` platform manifests | Not Started | Guide layout includes manifests, repo does not |
| VG-05 | Sections 2 and 16 | `infra/helm-charts/helmix` self-deployment chart | Not Started | Guide layout includes chart, repo does not |
| VG-06 | Section 11 Phase 4.4 | K8s NetworkPolicies, ServiceAccounts/RBAC, ResourceQuotas, restricted PSA artifacts | Not Started | Security hardening claims exceed current repo artifacts |
| VG-07 | Section 12 Dashboard | Missing dashboard routes: logs, deployments, pipelines, cost, settings | Not Started | Current app exposes only dashboard root, incidents, observability, login |
| VG-08 | Section 12 Dashboard | Live deployment WebSocket UI and SSE log streaming | Not Started | Gateway has `/ws` proxy, but no frontend implementation or log page |
| VG-09 | Section 13 Testing Strategy | Playwright E2E suite | Not Started | Repo uses Go e2e only at present |
| VG-10 | Section 13 Testing Strategy | k6 load test scripts | Not Started | `tests/load` is effectively empty |
| VG-11 | Section 13.1 | incident-ai eval harness and golden cases | Not Started | `tests/eval` absent |
| VG-12 | Section 16.3 | `.github/workflows/platform-deploy.yml` and release stages | Not Started | Only `ci.yml` exists |
| VG-13 | Sections 13 and 16.3 | CI gates for mypy, Semgrep, Checkov, staging smoke, and prod approval path | Not Started | Current CI does not enforce these stages |

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
- [x] Pipeline generator workflow generation and gateway integration complete
- [x] Deployment engine blue-green and rollback complete
- [x] Phase 2 unit and e2e tests passing

### Phase 3 Checklist

- [x] Observability metric pipeline and alerting rules complete
- [x] Incident AI diagnosis and context gathering complete
- [x] Auto-remediation workflow complete
- [x] Qdrant memory integration complete
- [x] Phase 3 unit and e2e tests passing

### Phase 4 Checklist

- [x] Multi-tenancy and RBAC org endpoints implemented (POST /orgs, POST /orgs/invite, POST /orgs/accept-invite, GET /orgs/members, PATCH /orgs/members/{user_id}, DELETE /orgs/members/{user_id})
- [x] RequireRole middleware tested for all role scenarios (allow/deny/unauthenticated)
- [x] Gateway proxy for /api/v1/orgs → auth-service
- [x] org_invites migration (000009) applied
- [x] Vault secret management complete
- [x] Terraform modules for cloud providers complete
- [x] Security hardening controls complete
- [x] Full CLI command set complete
- [x] Production readiness checklist passed

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
| 2026-03-13 | Completed Next Action 1 by extending incidents deep-link smoke coverage in `frontend/dashboard/app/dashboard/incidents/presets.test.ts` to validate `/dashboard/incidents?project_id=...&alert_rule=...` and `rule` alias mapping to suggested action presets; validated with `cd frontend/dashboard && npm run test` (7/7 pass) |
| 2026-03-13 | Completed Next Action 2 by expanding api-gateway incidents list integration assertions to validate full response shape fields (`project_id`, `alert_id`, `created_at`, `ai_diagnosis.root_cause`, `ai_diagnosis.confidence`) in `services/api-gateway/internal/gateway/gateway_integration_test.go`; validated with `cd services/api-gateway && go test ./...` |
| 2026-03-13 | Completed Next Action 3 by adding CI job `phase3-incident-e2e-smoke` in `.github/workflows/ci.yml` (migrate -> refresh `incident-ai` image -> `make test-e2e-phase3-incident`), and locally prevalidated compose flow with `make test-e2e-phase3-incident` pass |
| 2026-03-13 | Collected local reliability baseline for `make test-e2e-phase3-incident` over 3 consecutive runs after CI wiring: pass/pass/pass with wall times 88.89s, 102.47s, 91.50s (no local flakes observed) |
| 2026-03-13 | Refined CI Phase 3 incident refresh step from no-op `docker compose build incident-ai` to `docker compose pull incident-ai` (service uses `image: python:3.12-slim`); validated revised sequence with local `docker compose pull incident-ai && make test-e2e-phase3-incident` pass |
| 2026-03-13 | Added first-run CI observability instrumentation for `phase3-incident-e2e-smoke`: job timeout guard, compose service-state summary, and elapsed runtime summary in `GITHUB_STEP_SUMMARY` to speed flake triage after push/PR |
| 2026-03-13 | Implemented incident-ai auto-remediation execution path for `auto_execute=true` (executes recommended/fallback actions, appends action history, publishes `autoheal.triggered` with `source=auto`), added regression test `test_auto_execute_runs_actions_and_publishes_autoheal`, and validated with containerized `pytest tests -q` (16/16 pass) plus `make test-e2e-phase3-incident` pass |
| 2026-03-13 | Implemented Qdrant memory integration in incident-ai with collection bootstrap, deterministic local embeddings, top-3 similar incident retrieval for diagnosis and `/incidents/{id}/similar`, plus incident memory upsert after processing; validated with containerized `pytest tests -q` (18/18 pass) and `make test-e2e-phase3-incident` pass |
| 2026-03-13 | Added CI artifact capture for `phase3-incident-e2e-smoke` (`compose-ps.txt` and `compose-logs.txt`) so the first external GitHub Actions run has preserved diagnostics even when the job flakes or fails |
| 2026-03-13 | First GitHub Actions run of `phase3-incident-e2e-smoke` and `phase1-e2e-smoke` failed because `incident-ai` required Qdrant at startup but compose did not declare that dependency; fixed by adding `qdrant: service_healthy` to `incident-ai.depends_on`, then cold-start revalidated local `make test-e2e-phase1` and `make test-e2e-phase3-incident` pass |
| 2026-03-13 | Reduced CI noise for rerun by opting workflow into Node 24 action runtime and setting `actions/setup-go` cache dependency path to `${matrix.module}/go.sum`, removing module-cache warnings caused by missing root `go.sum` |
| 2026-03-13 | GitHub Actions run #5 passed all jobs (Success, 3m 46s, 1 artifact): 9 Go test matrix jobs, Python tests, Frontend production build, Phase 1 E2E Smoke (2m 56s), Phase 3 Incident E2E Smoke (2m 52s); Phase 3 marked Done at 100% |
| 2026-03-13 | Re-ran fresh local Phase 1 -> Phase 3 validation (`make test-e2e-phase1`, `make test-e2e-phase2-flow`, `make test-e2e-phase3-observability`, `make test-e2e-phase3-incident`): all pass; added `phase2-e2e-smoke` to `.github/workflows/ci.yml` and relaxed observability compose healthcheck timing to tolerate cold-start `go run` dependency downloads |
| 2026-03-13 | Started Phase 4: implemented Phase 4.1 Multi-tenancy & RBAC — org_invites migration (000009), org store methods (CreateOrg/GetOrgMembers/CreateInvite/AcceptInvite/UpdateMemberRole/RemoveMember), six org endpoints on auth-service with RequireRole guards, /api/v1/orgs gateway proxy, RequireRole middleware tests (allow/deny/unauthenticated), gateway integration tests for orgs create/list/unauthenticated; all tests pass (`libs/auth`, `auth-service`, `api-gateway`) |
| 2026-03-14 | Continued Phase 4.2: implemented Vault AppRole-backed KV client in auth-service, added owner/admin-protected secrets CRUD endpoints (`POST /secrets`, `GET /secrets/{service}/{key}`, `DELETE /secrets/{service}/{key}`), wired gateway proxy route `/api/v1/secrets`, added auth-service integration tests for success/permission-denied/vault-unavailable and gateway proxy tests; validated with `cd services/auth-service && go test ./...` and `cd services/api-gateway && go test ./...` |
| 2026-03-14 | Completed Phase 4.2 operationalization: added `vault-bootstrap` compose init service with scripted AppRole/policy bootstrap (`scripts/bootstrap-vault-approle.sh`), added compose smoke target `make test-e2e-phase4-vault`, and validated end-to-end secrets CRUD via gateway with `TestPhase4VaultSecretsCRUDViaGateway` pass |
| 2026-03-14 | Started Phase 4.4 security hardening slice: added gateway security headers middleware (CSP, HSTS, X-Frame-Options, nosniff, referrer policy), tightened auth-service secret path/value validation, added integration tests for both services, and revalidated with `cd services/api-gateway && go test ./...`, `cd services/auth-service && go test ./...`, and `make test-e2e-phase4-vault` |
| 2026-03-14 | Extended Phase 4.4 security hardening: implemented gateway request-body size limits (path-aware caps), method-aware rate limiting (read/write buckets), fixed rate limiter ZSET member collision under burst traffic, added gateway tests for write-limit and oversized payload rejection, wired non-blocking Trivy scan target (`make security-scan-trivy`) plus CI job, and validated with `cd services/api-gateway && go test ./...` and `make test-e2e-phase4-vault` |
| 2026-03-14 | Completed Phase 4.4 security hardening: upgraded vulnerable dependencies (`github.com/golang-jwt/jwt/v5` to `v5.2.2`, `golang.org/x/crypto` to `v0.35.0`, `next`/`eslint-config-next` to `14.2.35`), added `.trivyignore` risk-acceptance entries for local dev JWT key and one Next.js advisory requiring 15.x+, and revalidated with `make security-scan-trivy` (actionable HIGH/CRITICAL findings reduced to 0) plus `make test-e2e-phase4-vault` |
| 2026-03-14 | Completed Phase 4.3 Terraform scaffolding: added `infra/terraform` multi-cloud modules (`vpc`, `kubernetes`, `database`, `cache`, `registry`) with common `cloud_provider` interface, added environment stacks (`dev`, `staging`, `production`), wired `make tf-plan`, `make tf-apply`, and `make tf-destroy`, and validated with `make tf-plan env=dev`, `make tf-plan env=staging`, and `make tf-plan env=production` |
| 2026-03-14 | Completed Phase 4.5 full CLI: replaced bootstrap CLI with Cobra+Viper command tree (`health`, `auth`, `orgs`, `secrets`, `repos`, `infra`, `pipelines`, `deployments`, `observability`, `incidents`), added docs in `cli/helmix-cli/README.md`, added root `make cli-build` for linux/darwin/windows outputs, and validated with `cd cli/helmix-cli && go test ./... && go build ./cmd/helmix` plus `make cli-build` (artifacts in `dist/`) |
| 2026-03-14 | Completed Phase 4 production readiness closure: fixed Vault AppRole bootstrap idempotency in `scripts/bootstrap-vault-approle.sh` for repeated compose runs, then validated all readiness gates with `make security-scan-trivy`, `make test-e2e-phase1`, `make test-e2e-phase2-flow`, `make test-e2e-phase3-observability`, `make test-e2e-phase3-incident`, `make test-e2e-phase4-vault`, `make cli-build`, and `make tf-plan env=production` |
| 2026-03-14 | Improved root test reliability: fixed `make test` to run Go tests module-by-module (go.work-compatible) and run incident-ai tests in Docker (`python:3.12-slim`) to avoid host `pytest` dependency drift; validated with successful `make test` run (all selected Go modules pass; incident-ai `18 passed`) |
| 2026-03-14 | Improved lint reliability and closed remaining static-analysis findings: rewired `make lint` to containerized `golangci-lint`/`ruff`/frontend `next lint`, fixed transaction rollback `errcheck` issues in auth-service/repo-analyzer stores, removed no-op self-assignment in repo-analyzer classifier, cleaned unused incident-ai imports, and validated with green `make lint`, green `make security-scan-trivy`, and post-fix `make test` pass |
| 2026-03-14 | Re-baselined tracker against guide-reality gaps: marked Phase 4 back to In Progress, added verified guide-alignment backlog (VG-01..VG-13), and switched active sprint focus to completion-gap closure |
| 2026-03-14 | Started GAP-01 audit logging: added migration `000010_create_audit_logs`, implemented `CreateAuditLog` in auth-service store, wired login/refresh/logout audit events in auth-service handlers, and validated with `cd services/auth-service && go test ./...`; recovered local migration state by forcing version 10 then re-running `make migrate` to green (`no change`) |
| 2026-03-14 | Continued GAP-01 audit logging: refactored auth-service server dependencies behind interfaces, added route-level audit tests for refresh/logout persistence, and expanded audit events for GitHub repo listing and repo-analysis success/failure paths; revalidated with `cd services/auth-service && go test ./...` |
| 2026-03-14 | Expanded GAP-01 audit coverage across org-management and secrets handlers, added route-level tests for org creation success and secret read failure, and revalidated with `cd services/auth-service && go test ./...` |
| 2026-03-14 | Started GAP-02 deploy risk controls: added migration `000011_add_deployment_scan_fields`, threaded `scan_results` and `accept_risk` through deployment-engine request/store/response models, enforced `accept_risk` when high/critical findings are present, and validated with `cd services/deployment-engine && go test ./...` plus gateway forwarding coverage via `cd services/api-gateway && go test ./internal/gateway -run TestDeploymentStartProxyAuthorized` |
| 2026-03-14 | Completed GAP-02 verification: added Phase 2 e2e risk override test (`TestPhase2GatewayDeploymentRiskOverride`), force-recreated deployment-engine and api-gateway containers, validated rejection without `accept_risk` and acceptance with override, and applied migration `000011` via `make migrate` (`11/u add_deployment_scan_fields`) |

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
| Notes | Full analyze -> infra -> pipeline -> deploy -> rollback flow passes end-to-end through the gateway on fresh local rerun; deployment-engine module, proxy, compose smoke, CI matrix coverage, and dedicated `phase2-e2e-smoke` workflow coverage are in place. |

## Next Actions

1. Close GAP-01 by deciding whether the current mixed `auth.*`, `org.*`, and `secret.*` events should be normalized into a single audit taxonomy or intentionally remain service-scoped, then document that choice in the tracker and guide.
2. Start GAP-03 by scaffolding `services/notification-service` and wiring its initial event subscriptions into compose and CI.
3. Add missing deployment artifacts under `infra/kubernetes` and `infra/helm-charts/helmix`, including the first security resources (NetworkPolicies, ServiceAccounts/RBAC, ResourceQuotas).
4. Expand the dashboard and test/release surfaces to match the guide: missing routes, WS/SSE flows, Playwright, k6, eval harness, and platform-deploy workflow.
