.PHONY: dev api web db-up db-down db-logs build tidy test vet lint clean migrate-up migrate-down migrate-status

# -----------------------------------------------------------------------------
# Setup / install
# -----------------------------------------------------------------------------
install: ## Install all dependencies (Go + Node)
	cd apps/api && go mod download
	cd apps/web && npm install

# -----------------------------------------------------------------------------
# Development
# -----------------------------------------------------------------------------
dev: db-up ## Run API + web in development
	@set -e; \
	trap 'kill $$API_PID' EXIT; \
	cd apps/api && go run . & API_PID=$$!; \
	cd apps/web && npm run dev

api: ## Run Go API only
	cd apps/api && go run .

web: ## Run Next.js web only
	cd apps/web && npm run dev

# -----------------------------------------------------------------------------
# Database
# -----------------------------------------------------------------------------
db-up: ## Start PostgreSQL
	docker compose up -d postgres

db-down: ## Stop PostgreSQL
	docker compose down

db-logs: ## Tail PostgreSQL logs
	docker compose logs -f postgres

migrate-up: ## Apply all pending database migrations
	cd apps/api && go run ./cmd/migrate up

migrate-down: ## Roll back one database migration
	cd apps/api && go run ./cmd/migrate down

migrate-status: ## Show current database migration version
	cd apps/api && go run ./cmd/migrate status

# -----------------------------------------------------------------------------
# Build / test / lint
# -----------------------------------------------------------------------------
build: ## Build API and web
	cd apps/api && go build ./...
	cd apps/web && npm run build

test: ## Run all tests
	cd apps/api && go test ./...
	cd apps/web && npm run test

tidy: ## Tidy Go modules
	cd apps/api && go mod tidy

vet: ## Vet Go code
	cd apps/api && go vet ./...

lint: ## Lint API and web
	$(MAKE) vet
	cd apps/web && npm run lint

# -----------------------------------------------------------------------------
# Cleanup
# -----------------------------------------------------------------------------
clean: ## Remove build artifacts
	cd apps/api && go clean
	cd apps/web && rm -rf .next out

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "%-12s %s\n", $$1, $$2}'
