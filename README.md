# Quorum

A high-concurrency Q&A serving engine built on the Stack Exchange corpus.
Designed as a single-deployable backend service that serves a Stack Overflow–scale question-and-answer corpus.

> Note: This dataset includes the Stack Exchange Community Data Dump. Content is licensed under CC BY-SA (Stack Exchange Network).

## Quick start

1. Start PostgreSQL and Redis: `make up`
2. Start the API: `make api`
3. The API is available at `http://localhost:8080` — liveness at `/healthz`, readiness at `/readyz`.

## Layout

- `backend/cmd/api` — HTTP Server
- `backend/cmd/worker` — Outbox relay + job consumers
- `backend/cmd/ingest` — Bulk loader CLI
- `backend/cmd/verify` — Invariant checker
- `backend/internal/domain/<capability>` — pure domain entities, value objects, and rules
- `backend/internal/usecase/<capability>` — application services with their commands and consumer-owned ports
- `backend/internal/adapter/` — HTTP transport plus PostgreSQL and Redis adapters grouped by capability
- `backend/internal/infrastructure/` — configuration, clients, logging, and dependency injection
- `frontend/` — React frontend application using Vite and TypeScript
- `test/` — Playwright API integration and browser end-to-end project

## Validation

Run `make test` for backend and frontend unit tests. Run `make backend-check` for Go formatting, vetting, and tests. Start Docker with `make up`, then run `make test-integration` for API integration coverage and `make test-e2e` for browser end-to-end coverage. See [`test/README.md`](test/README.md) for setup and test-environment overrides.
