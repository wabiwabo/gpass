VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)

.PHONY: dev setup up down test lint build cover keys

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

test: ## Run all tests
	cd apps/bff && go test ./... -count=1
	turbo test

test-verbose: ## Run all tests with verbose output
	cd apps/bff && go test ./... -v -count=1

cover: ## Run tests with coverage report
	cd apps/bff && go test ./... -coverprofile=coverage.out && go tool cover -func=coverage.out

lint: ## Run linters
	cd apps/bff && go vet ./...
	turbo lint

keys: ## Generate OIDC client EC P-256 key pair
	./tools/scripts/generate-keys.sh

docker-build: ## Build BFF Docker image with version info
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-t garudapass/bff:$(VERSION) \
		-t garudapass/bff:latest \
		apps/bff

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
