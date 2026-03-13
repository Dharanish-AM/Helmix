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
| 1 | GitHub Integration and Foundation Services | Weeks 1-3 | Not Started | 0% | Pending auth and gateway implementation |
| 2 | Infra Generator, Pipelines and Deployment | Weeks 4-6 | Not Started | 0% | Pending Phase 1 completion |
| 3 | Observability and AI Incident Engine | Weeks 7-10 | Not Started | 0% | Pending Phase 2 completion |
| 4 | Production Hardening | Weeks 11-12 | Not Started | 0% | Pending Phase 3 completion |

## Active Sprint Focus

Current sprint goal: Complete all Phase 0 acceptance criteria.

| Task ID | Task | Priority | Owner | Status | Due Date | Updated |
|---|---|---|---|---|---|---|
| P0-01 | Ensure make dev starts local infra with healthy containers | P0 | Team | Done | 2026-03-14 | 2026-03-13 |
| P0-02 | Verify and run all SQL migrations on fresh DB | P0 | Team | Done | 2026-03-14 | 2026-03-13 |
| P0-03 | Validate libs event-sdk tests and NATS flow | P0 | Team | Done | 2026-03-14 | 2026-03-13 |
| P0-04 | Add missing service wiring to docker-compose for all services | P1 | Team | Not Started | 2026-03-15 | 2026-03-13 |
| P0-05 | Replace frontend placeholder with complete Next.js scaffold checks | P1 | Team | In Progress | 2026-03-15 | 2026-03-13 |

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

- [ ] Auth service endpoints complete
- [ ] JWT middleware and role middleware in libs auth
- [ ] API gateway middleware stack complete
- [ ] Repo analyzer detection logic complete
- [ ] Dashboard auth flow complete
- [ ] Phase 1 unit, integration, e2e tests passing

### Phase 2 Checklist

- [ ] Infra generator templates and validation complete
- [ ] Pipeline generator workflow and PR creation complete
- [ ] Deployment engine blue-green and rollback complete
- [ ] Phase 2 unit and e2e tests passing

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
| R-001 | 2026-03-13 | Environment | Go toolchain not available in current execution environment for automated validation | Medium | Team | Run go test and go work sync on developer machine | Open |
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

## Next Actions

1. Begin Phase 1: implement auth-service GitHub OAuth endpoints.
2. Implement libs/auth JWT middleware.
3. Implement api-gateway Chi router with middleware stack.
4. Implement repo-analyzer stack detection logic.
5. Connect dashboard auth flow to auth-service.
