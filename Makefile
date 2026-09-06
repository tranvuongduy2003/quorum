.PHONY: up down api worker ingest-help ingest-small verify bench test test-integration test-e2e openapi openapi-lint backend-check frontend-dev frontend-build frontend-lint frontend-preview frontend-test frontend-check probe ready db-shell cache-shell logs migrate-ingest quarantine-count

up:
	docker compose up -d

down:
	docker compose down

api:
	cd backend && go run ./cmd/api

probe:
	curl -i http://localhost:8080/healthz

ready:
	curl -i http://localhost:8080/readyz

db-shell:
	docker compose exec postgres psql -U app -d app

cache-shell:
	docker compose exec redis redis-cli

logs:
	docker compose logs -f

worker:
	cd backend && go run ./cmd/worker

ingest-help:
	cd backend && go run ./cmd/ingest --help

ingest-small:
	cd backend && go run ./cmd/ingest --site academia.stackexchange.com --archive ../data/academia.stackexchange.com.7z --tables posts --dry-run

openapi:
	cd backend && go run github.com/swaggo/swag/v2/cmd/swag@v2.0.0-rc5 init --v3.1 -g openapi.go -d ./internal/adapter/http -o ./docs --ot yaml

openapi-lint:
	npx --yes @redocly/cli@latest lint --extends=minimal backend/docs/swagger.yaml

verify:
	cd backend && go run ./cmd/verify

bench:
	@echo "Running k6 benchmarks..."

test:
	cd backend && go test ./...
	cd backend && go vet ./...
	cd frontend && npm run test -- --run

backend-check:
	cd backend && gofmt -w . && go vet ./... && go test ./...

test-integration:
	cd test && npm run test:integration

test-e2e:
	cd test && npm run test:e2e

frontend-dev:
	cd frontend && npm run dev

frontend-build:
	cd frontend && npm run build

frontend-lint:
	cd frontend && npm run lint

frontend-preview:
	cd frontend && npm run preview

frontend-test:
	cd frontend && npm run test -- --run

frontend-check:
	cd frontend && npm run lint && npm run typecheck && npm run test -- --run

migrate-ingest:
	docker compose exec -T postgres sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -v ON_ERROR_STOP=1 -f /migrations/000001_ingest_quarantine.sql'

quarantine-count:
	@docker compose exec -T postgres sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -Atc "SELECT count(*) FROM ingest_quarantine;"'
