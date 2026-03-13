---
description: "Use for all Helmix development tasks to enforce architecture contracts, phase gating, and tracker updates aligned to the implementation guide."
applyTo: "**"
---

# Helmix AI Instructions

These instructions apply to all work in this repository.

## Source of Truth

- Always use Helmix_Full_Implementation_Guide.md as the primary specification.
- If implementation conflicts with assumptions, follow the guide.
- Keep PROJECT_TRACKER.md updated after meaningful work sessions.

## Delivery Model

- Follow phase order strictly: do not start Phase N+1 before Phase N acceptance criteria pass.
- Build vertical slices that are runnable and testable.
- Prefer incremental, reviewable commits aligned to a single phase task.

## Architecture Rules

- Treat NATS JetStream event schemas as contracts.
- Default to event-driven service communication.
- Avoid adding direct inter-service HTTP unless explicitly required by the guide.
- Keep deployment and blue-green state in PostgreSQL, not in-memory.

## Service Standards

- Every Go service must expose /health returning:
	{"status":"ok","service":"<name>","version":"0.1.0"}
- Validate environment variables via config structs at startup.
- Wrap errors with context using fmt.Errorf("...: %w", err).
- Use structured logging (slog for Go, structlog for Python).
- Use context timeouts for network, database, and Kubernetes operations.

## Security and Secrets

- Never hardcode secrets, keys, or tokens.
- Use environment variables and placeholders in docs and examples.
- Prefer Vault-based secret patterns from the guide.

## Testing Expectations

- Do not skip tests for the current phase.
- Add or update tests together with code changes.
- For event changes, verify producer and consumer contract compatibility.
- Keep Phase acceptance checks current in PROJECT_TRACKER.md.

## Tracker Discipline

- For each completed task, update PROJECT_TRACKER.md with:
	- task status change
	- acceptance evidence (command or result)
	- a session update log line
- Keep tracker section names and scope aligned with the guide sections.

## Code Change Style

- Keep edits focused; avoid unrelated refactors.
- Preserve existing naming, folder layout, and module boundaries.
- Prefer readability and explicit behavior over clever abstractions.