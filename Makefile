.PHONY: dev setup up down test lint

setup: ## First-time setup
	pnpm install
	cd apps/bff && go mod download
	chmod +x tools/scripts/*.sh
	cp .env.example .env

up: ## Start all services via Docker Compose
	docker compose up -d

down: ## Stop all services
	docker compose down

dev: ## Start development servers
	turbo dev

test: ## Run all tests
	cd apps/bff && go test ./...
	turbo test

lint: ## Run linters
	cd apps/bff && go vet ./...
	turbo lint

keys: ## Generate OIDC client EC P-256 key pair
	./tools/scripts/generate-keys.sh
