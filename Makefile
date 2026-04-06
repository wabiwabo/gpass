VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)

.PHONY: dev setup up down test lint build cover keys test-race test-count docker-build-bff load-smoke load-stress load-soak migrate vet

setup: ## First-time setup
	pnpm install
	cd apps/bff && go mod download
	chmod +x tools/scripts/*.sh
	cp -n .env.example .env || true

up: ## Start all services via Docker Compose
	docker compose up -d

down: ## Stop all services
	docker compose down

dev: ## Start development servers
	turbo dev

build: ## Build BFF binary with version info
	cd apps/bff && CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o ../../dist/bff .

GO_SERVICES := apps/bff services/identity services/garudainfo services/dukcapil-sim services/ahu-sim services/oss-sim services/garudacorp services/signing-sim services/garudasign services/garudaportal services/garudaaudit services/garudanotify packages/golib

test: ## Run all Go tests
	@total=0; \
	for svc in $(GO_SERVICES); do \
		echo "Testing $$svc..."; \
		cd $(CURDIR)/$$svc && go test ./... -count=1 || exit 1; \
	done
	@echo "All Go tests passed"

test-verbose: ## Run all Go tests with verbose output
	@for svc in $(GO_SERVICES); do \
		echo "=== $$svc ==="; \
		cd $(CURDIR)/$$svc && go test ./... -v -count=1 || exit 1; \
	done

test-race: ## Run all Go tests with race detector
	@for svc in $(GO_SERVICES); do \
		echo "Testing (race) $$svc..."; \
		cd $(CURDIR)/$$svc && go test ./... -race -count=1 || exit 1; \
	done

cover: ## Run tests with coverage report
	@for svc in $(GO_SERVICES); do \
		echo "Coverage $$svc..."; \
		cd $(CURDIR)/$$svc && go test ./... -coverprofile=coverage.out -covermode=atomic && go tool cover -func=coverage.out | tail -1; \
	done

test-count: ## Count all tests across services
	@total=0; \
	for svc in $(GO_SERVICES); do \
		count=$$(cd $(CURDIR)/$$svc && go test ./... -v -count=1 2>&1 | grep -c "^--- PASS:"); \
		total=$$((total + count)); \
		echo "$$svc: $$count tests"; \
	done; \
	echo ""; \
	echo "TOTAL: $$total tests"

lint: ## Run linters
	cd apps/bff && go vet ./...
	turbo lint

keys: ## Generate OIDC client EC P-256 key pair
	./tools/scripts/generate-keys.sh

docker-build: ## Build all Docker images
	@for svc in apps/bff services/identity services/garudainfo services/dukcapil-sim services/ahu-sim services/oss-sim services/garudacorp services/signing-sim services/garudasign services/garudaportal; do \
		name=$$(basename $$svc); \
		echo "Building $$name..."; \
		docker build -t garudapass/$$name:$(VERSION) -t garudapass/$$name:latest $$svc; \
	done

docker-build-bff: ## Build BFF Docker image only
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t garudapass/bff:$(VERSION) \
		-t garudapass/bff:latest \
		apps/bff

load-smoke: ## Run k6 smoke tests
	k6 run tests/load/smoke.js

load-stress: ## Run k6 stress tests
	k6 run tests/load/stress.js

load-soak: ## Run k6 soak tests (30 min)
	k6 run --duration 30m tests/load/soak.js

migrate: ## Run database migrations
	@for f in infrastructure/db/migrations/*.sql; do \
		echo "Applying: $$f"; \
		psql $$DATABASE_URL -f "$$f" 2>/dev/null || true; \
	done
	@echo "Migrations complete"

vet: ## Run go vet on all services
	@for svc in $(GO_SERVICES); do \
		cd $(CURDIR)/$$svc && go vet ./...; \
	done

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
