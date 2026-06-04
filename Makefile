# =============================================================================
# Flagstone
# =============================================================================
.PHONY: help build run test test-int test-int-v test-cover lint fmt migrate migrate-down migrate-create setup seed seed-build clean docker-build docker-run

BINARY := bin/flagstone
SEED_BINARY := bin/seed
MODULE := github.com/flagstonehq/flagstone

DATABASE_URL ?= postgres://flagstone:flagstone_dev@localhost:5432/flagstone?sslmode=disable

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# -----------------------------------------------------------------------------
# Development (Build, run and test the server)
# -----------------------------------------------------------------------------

build:
	go build -o $(BINARY) ./cmd/flagstone

run:
	go run ./cmd/flagstone

seed-build:
	go build -o $(SEED_BINARY) ./cmd/seed

fmt:
	go fmt ./...
	goimports -w .

lint:
	golangci-lint run ./...

vet:
	go vet ./...

# -----------------------------------------------------------------------------
# Testing (Run unit tests, integration tests and generate coverage report)
# -----------------------------------------------------------------------------

test:
	go test -race -short ./...

# Same as test but with the integration tier (no -short). Use in CI / pre-release.
test-race:
	go test -race -count=1 ./...

# Benchmarks for the SDK. Filters out non-benchmark output by default.
bench:
	go test -run=^$$ -bench=. -benchmem ./pkg/sdk/...

# Integration tests. Packages run in parallel safely thanks to per-package
# Postgres schema isolation in internal/testutil/pgtest — each package owns
# its own schema and migrations run there.
test-int:
	go test -race -count=1 ./...

# Same as test-int, but verbose. Use during handler development to see which
# tests ran and what failed.
test-int-v:
	go test -race -count=1 -v ./...

test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# -----------------------------------------------------------------------------
# Database (Run Database migrations up, rollback last migration and create a new migration)
# -----------------------------------------------------------------------------

migrate:
	@command -v migrate >/dev/null 2>&1 || { echo "Install golang-migrate: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"; exit 1; }
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	@command -v migrate >/dev/null 2>&1 || { echo "Install golang-migrate: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"; exit 1; }
	migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-create:
	@command -v migrate >/dev/null 2>&1 || { echo "Install golang-migrate"; exit 1; }
	migrate create -ext sql -dir migrations -seq $(NAME)

# -----------------------------------------------------------------------------
# Docker
# -----------------------------------------------------------------------------

setup: ## Start only Postgres + Redis (for local Go development)
	docker compose up -d postgres redis
	@echo "Waiting for Postgres..."
	@until docker compose exec postgres pg_isready -U flagstone >/dev/null 2>&1; do sleep 1; done
	@echo "Dependencies ready. Run 'make migrate && make run' to start developing."

demo: ## Start the full stack (postgres, redis, migrate, api, web) + seed demo data
	docker compose up -d
	docker compose --profile seed run --rm seed
	@echo ""
	@echo "Dashboard: http://localhost:3000"
	@echo "API:       http://localhost:8080"
	@echo "Login:     admin@acme.com / password123"

down: ## Stop all services
	docker compose down

seed: seed-build ## Populate dev database with demo data (server must be running on :8080)
	@./$(SEED_BINARY)

clean: ## Stop all services and delete volumes (fresh DB)
	docker compose down -v

docker-build: ## Build the Docker image
	docker build -t flagstone:latest .

docker-run: ## Run the Docker image standalone (needs external Postgres + Redis)
	docker run --rm -p 8080:8080 \
		-e DATABASE_URL="$(DATABASE_URL)" \
		-e REDIS_URL="redis://localhost:6379" \
		-e JWT_SECRET="change-me-in-production-min-32-chars" \
		flagstone:latest
