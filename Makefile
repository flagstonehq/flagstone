# =============================================================================
# Flagstone — Makefile
# =============================================================================
# Common development commands. Run `make help` to see all targets.
# =============================================================================

.PHONY: help build run test test-int lint fmt migrate migrate-down setup clean docker-build docker-run

# Default Go binary output
BINARY := bin/flagstone
MODULE := github.com/thomas-vilte/flagstone

# Database connection for local dev (matches docker-compose.yml)
DATABASE_URL ?= postgres://flagstone:flagstone_dev@localhost:5432/flagstone?sslmode=disable

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# -----------------------------------------------------------------------------
# Development
# -----------------------------------------------------------------------------

build: ## Build the server binary
	go build -o $(BINARY) ./cmd/flagstone

run: ## Run the server locally (hot reload with go run)
	go run ./cmd/flagstone

fmt: ## Format all Go files
	go fmt ./...
	goimports -w .

lint: ## Run linters (requires golangci-lint)
	golangci-lint run ./...

vet: ## Run go vet
	go vet ./...

# -----------------------------------------------------------------------------
# Testing
# -----------------------------------------------------------------------------

test: ## Run unit tests
	go test -race -short ./...

test-int: ## Run integration tests (requires running Postgres + Redis)
	go test -race -count=1 ./...

test-cover: ## Run tests with coverage report
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# -----------------------------------------------------------------------------
# Database
# -----------------------------------------------------------------------------

migrate: ## Run database migrations up
	@command -v migrate >/dev/null 2>&1 || { echo "Install golang-migrate: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"; exit 1; }
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down: ## Rollback last migration
	@command -v migrate >/dev/null 2>&1 || { echo "Install golang-migrate: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"; exit 1; }
	migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-create: ## Create a new migration (usage: make migrate-create NAME=add_something)
	@command -v migrate >/dev/null 2>&1 || { echo "Install golang-migrate"; exit 1; }
	migrate create -ext sql -dir migrations -seq $(NAME)

# -----------------------------------------------------------------------------
# Docker
# -----------------------------------------------------------------------------

setup: ## Start local dev dependencies (Postgres + Redis)
	docker compose up -d
	@echo "Waiting for Postgres..."
	@until docker compose exec postgres pg_isready -U flagstone >/dev/null 2>&1; do sleep 1; done
	@echo "Dependencies ready. Run 'make migrate' to initialize the database."

down: ## Stop local dev dependencies
	docker compose down

clean: ## Stop dependencies and delete volumes (fresh database)
	docker compose down -v

docker-build: ## Build the Docker image
	docker build -t flagstone:latest .

docker-run: ## Run the Docker image
	docker run --rm -p 8080:8080 \
		-e DATABASE_URL="$(DATABASE_URL)" \
		flagstone:latest
