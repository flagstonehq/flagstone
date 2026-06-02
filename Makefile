# =============================================================================
# Flagstone
# =============================================================================
.PHONY: help build run test test-int test-int-v test-cover lint fmt migrate migrate-down migrate-create setup seed clean docker-build docker-run

BINARY := bin/flagstone
MODULE := github.com/thomas-vilte/flagstone

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
# Docker (Postgres + Redis)
# -----------------------------------------------------------------------------

setup:
	docker compose up -d
	@echo "Waiting for Postgres..."
	@until docker compose exec postgres pg_isready -U flagstone >/dev/null 2>&1; do sleep 1; done
	@echo "Dependencies ready. Run 'make migrate' to initialize the database."

down:
	docker compose down

seed: ## Populate dev database with demo data (server must be running on :8080)
	@bash scripts/seed.sh

clean:
	docker compose down -v

docker-build:
	docker build -t flagstone:latest .

docker-run:
	docker run --rm -p 8080:8080 \
		-e DATABASE_URL="$(DATABASE_URL)" \
		flagstone:latest
