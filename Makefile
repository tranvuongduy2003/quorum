.PHONY: up down api worker ingest-help ingest-small guard-ingest-writes verify bench bench-copy test test-integration test-e2e openapi openapi-lint backend-check frontend-dev frontend-build frontend-lint frontend-preview frontend-test frontend-check probe ready db-shell cache-shell logs migrate-ingest migrate-copy migrate-copy-benchmark migrate-checkpoint quarantine-count crash-trial-reference crash-trial-counts

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

guard-ingest-writes:
	cd backend && go run ./cmd/guardingest

openapi:
	cd backend && go run github.com/swaggo/swag/v2/cmd/swag@v2.0.0-rc5 init --v3.1 -g openapi.go -d ./internal/adapter/http -o ./docs --ot yaml

openapi-lint:
	npx --yes @redocly/cli@latest lint --extends=minimal backend/docs/swagger.yaml

verify:
	cd backend && go run ./cmd/verify

bench:
	@echo "Running k6 benchmarks..."

bench-copy:
	cd backend && go run ./cmd/benchcopy --rows 1000000 --repetitions 3 --output ../docs/benchmarks/SPEC-003-copy-results.md

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

migrate-copy:
	docker compose exec -T postgres sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -v ON_ERROR_STOP=1 -f /migrations/000002_corpus_tables.sql'

migrate-copy-benchmark:
	docker compose exec -T postgres sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -v ON_ERROR_STOP=1 -f /migrations/000003_copy_benchmark.sql'

migrate-checkpoint:
	docker compose exec -T postgres sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -v ON_ERROR_STOP=1 -f /migrations/000004_ingest_checkpoints.sql'

quarantine-count:
	@docker compose exec -T postgres sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -Atc "SELECT count(*) FROM ingest_quarantine;"'

crash-trial-reference:
	docker compose exec -T postgres sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -v ON_ERROR_STOP=1 -c "TRUNCATE posts, post_bodies, users, comments, votes, badges, tags, post_links, post_history, ingest_quarantine, ingest_checkpoints CASCADE;"'
	cd backend && go run ./cmd/ingest --site academia.stackexchange.com --archive ../data/academia.stackexchange.com.7z --tables posts,users,comments,votes,badges,tags,post_links --checkpoint-interval 1000

crash-trial-counts:
	@docker compose exec -T postgres sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -Atc "SELECT '\''posts'\'', count(*) FROM posts WHERE site = '\''academia.stackexchange.com'\'' UNION ALL SELECT '\''users'\'', count(*) FROM users WHERE site = '\''academia.stackexchange.com'\'' UNION ALL SELECT '\''comments'\'', count(*) FROM comments WHERE site = '\''academia.stackexchange.com'\'' UNION ALL SELECT '\''votes'\'', count(*) FROM votes WHERE site = '\''academia.stackexchange.com'\'' UNION ALL SELECT '\''badges'\'', count(*) FROM badges WHERE site = '\''academia.stackexchange.com'\'' UNION ALL SELECT '\''tags'\'', count(*) FROM tags WHERE site = '\''academia.stackexchange.com'\'' UNION ALL SELECT '\''post_links'\'', count(*) FROM post_links WHERE site = '\''academia.stackexchange.com'\'' UNION ALL SELECT '\''quarantine'\'', count(*) FROM ingest_quarantine WHERE site = '\''academia.stackexchange.com'\'' ORDER BY 1;"'
