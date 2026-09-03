# Quorum integration and end-to-end tests

This project verifies the running Quorum system with Playwright and owns every non-unit test. API integration tests cover the service boundary, PostgreSQL, Redis, routing, validation, request identity, and rate limiting. Browser tests cover the service console's critical workflows and its degraded API states.

## Prerequisites

- Node.js 22
- Go from `backend/go.mod`
- Docker Desktop or another Docker-compatible runtime

## Run locally

From the repository root:

```bash
make up
cd test
npm ci
npx playwright install chromium
npm run test
```

Playwright starts the API and Vite server, waits for `/readyz`, and stops both processes when the run finishes. Docker services remain running until `make down`.

## Commands

| Command | Scope |
|---|---|
| `npm run test` | API integration and browser end-to-end tests |
| `npm run test:integration` | API integration tests only |
| `npm run test:e2e` | Browser end-to-end tests only |
| `npm run test:headed` | Run tests in a visible Chromium window |
| `npm run test:ui` | Open Playwright's interactive test UI |
| `npm run report` | Open the most recent HTML report |
| `npm run typecheck` | Type-check the Playwright project |

## Existing environments

Set `PLAYWRIGHT_NO_WEB_SERVER=true` when the API and frontend are already running. Set `PLAYWRIGHT_API_BASE_URL` and `PLAYWRIGHT_BASE_URL` to target non-default endpoints. The target must have PostgreSQL and Redis available and use the documented debug and rate-limit settings.

## Artifacts

Failed tests retain traces, screenshots, and video under Playwright's report directories. Those outputs are ignored by Git and uploaded by CI when a run fails.
