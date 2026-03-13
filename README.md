# Helmix

[![CI](https://github.com/Dharanish-AM/Helmix/actions/workflows/ci.yml/badge.svg)](https://github.com/Dharanish-AM/Helmix/actions/workflows/ci.yml)

Helmix is a multi-service platform for repository analysis, infrastructure generation, and automated delivery workflows.

## Monorepo Layout

- `services/` - core backend services (`auth-service`, `api-gateway`, `repo-analyzer`, and Phase 2+ services)
- `libs/` - shared Go libraries (`auth`, `event-sdk`, `shared-utils`)
- `frontend/dashboard/` - Next.js dashboard
- `infra/migrations/` - PostgreSQL schema migrations
- `tests/e2e/` - end-to-end verification for platform flows

## Local Development

### Start local stack

```bash
make dev
```

### Run migrations

```bash
make migrate
```

### Run core tests

```bash
make test
```

### Run Phase 1 e2e smoke

```bash
make test-e2e-phase1
```

## CI

The workflow in `.github/workflows/ci.yml` runs on pushes to `main` and pull requests.

- Go tests for key modules and services
- Frontend production build (on PRs, only when `frontend/dashboard/**` changes)
- Phase 1 e2e smoke (`make test-e2e-phase1`)

On pull requests, backend and e2e jobs are path-filtered to skip unnecessary runs when unrelated files change. Pushes to `main` run the full pipeline.
