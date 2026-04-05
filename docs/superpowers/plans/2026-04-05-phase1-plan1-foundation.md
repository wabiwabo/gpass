# GarudaPass Phase 1, Plan 1: Foundation Infrastructure

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Set up the monorepo, local development environment, Keycloak with FAPI 2.0, Go BFF with session management, and production-ready infrastructure configs so all subsequent feature work has a solid foundation to build on.

**Architecture:** Turborepo monorepo with Go workspaces for backend services. Keycloak as the OIDC provider configured for FAPI 2.0 (PAR, DPoP, PKCE S256, private_key_jwt). Go BFF handles web sessions via Redis-backed HttpOnly cookies, proxying all auth flows. Docker Compose for local dev; Kubernetes Helm charts for staging/production.

**Tech Stack:** Go 1.22+, Keycloak 26+, PostgreSQL 16, Redis 7, Apache Kafka (via Redpanda for local dev), Kong 3.x, Next.js 14+, Turborepo, Docker Compose, Kubernetes/Helm

---

## File Structure

```
gpass/
├── turbo.json
├── package.json                          # Turborepo root
├── go.work                               # Go workspace
├── go.work.sum
├── docker-compose.yml                    # Local dev: all services
├── .env.example                          # Environment variables template
├── Makefile                              # Top-level commands
│
├── apps/
│   ├── bff/                              # Go BFF service
│   │   ├── go.mod
│   │   ├── go.sum
│   │   ├── main.go                       # Entry point, server setup
│   │   ├── config/
│   │   │   └── config.go                 # Environment-based configuration
│   │   ├── handler/
│   │   │   ├── auth.go                   # /auth/login, /auth/callback, /auth/logout
│   │   │   ├── auth_test.go
│   │   │   ├── session.go                # /auth/session (get current user)
│   │   │   ├── session_test.go
│   │   │   ├── proxy.go                  # Reverse proxy to backend services
│   │   │   └── proxy_test.go
│   │   ├── middleware/
│   │   │   ├── csrf.go                   # Double-submit cookie CSRF protection
│   │   │   ├── csrf_test.go
│   │   │   ├── session.go                # Session cookie validation middleware
│   │   │   └── session_test.go
│   │   ├── session/
│   │   │   ├── store.go                  # Redis session store interface + impl
│   │   │   └── store_test.go
│   │   ├── oidc/
│   │   │   ├── client.go                 # Keycloak OIDC client (PAR, PKCE, token exchange)
│   │   │   └── client_test.go
│   │   └── Dockerfile
│   │
│   ├── citizen-portal/                   # Next.js citizen-facing app
│   │   ├── package.json
│   │   ├── next.config.js
│   │   ├── tsconfig.json
│   │   ├── app/
│   │   │   ├── layout.tsx                # Root layout with auth provider
│   │   │   ├── page.tsx                  # Landing / dashboard
│   │   │   ├── login/
│   │   │   │   └── page.tsx              # Login page (redirects to BFF /auth/login)
│   │   │   └── profile/
│   │   │       └── page.tsx              # User profile (protected)
│   │   ├── components/
│   │   │   ├── auth-provider.tsx          # React context for auth state
│   │   │   └── protected-route.tsx        # Route guard component
│   │   └── lib/
│   │       └── api.ts                     # BFF API client (fetch wrapper)
│   │
│   └── admin-portal/                     # Next.js admin app (same structure, separate deploy)
│       ├── package.json
│       ├── next.config.js
│       ├── tsconfig.json
│       └── app/
│           ├── layout.tsx
│           ├── page.tsx                   # Admin dashboard
│           └── users/
│               └── page.tsx               # User management (protected)
│
├── services/
│   └── .gitkeep                          # Placeholder for Plan 2+ services
│
├── packages/
│   ├── ui/                               # Shared React component library
│   │   ├── package.json
│   │   ├── tsconfig.json
│   │   └── src/
│   │       ├── index.ts
│   │       └── button.tsx                # Example shared component
│   ├── contracts/                        # Shared TypeScript types
│   │   ├── package.json
│   │   ├── tsconfig.json
│   │   └── src/
│   │       ├── index.ts
│   │       ├── user.ts                   # User type definitions
│   │       └── session.ts                # Session type definitions
│   └── eslint-config/                    # Shared ESLint config
│       ├── package.json
│       └── index.js
│
├── infrastructure/
│   ├── keycloak/
│   │   ├── Dockerfile                    # Custom Keycloak image with themes/SPIs
│   │   ├── realm-export.json             # GarudaPass realm configuration
│   │   └── themes/
│   │       └── garudapass/
│   │           └── login/
│   │               └── theme.properties  # Custom login theme config
│   ├── kong/
│   │   └── kong.yml                      # Declarative Kong config
│   ├── kafka/
│   │   └── topics.sh                     # Topic creation script
│   └── kubernetes/
│       ├── base/
│       │   ├── namespace.yaml
│       │   ├── keycloak.yaml
│       │   ├── postgresql.yaml
│       │   ├── redis.yaml
│       │   ├── kafka.yaml
│       │   ├── kong.yaml
│       │   └── bff.yaml
│       └── overlays/
│           ├── dev/
│           │   └── kustomization.yaml
│           └── staging/
│               └── kustomization.yaml
│
└── tools/
    └── scripts/
        ├── setup.sh                      # First-time setup script
        ├── generate-keys.sh              # Generate EC P-256 key pairs for OIDC clients
        └── seed-keycloak.sh              # Import realm + create test users
```

---

## Task 1: Monorepo Scaffold

**Files:**
- Create: `turbo.json`
- Create: `package.json`
- Create: `Makefile`
- Create: `go.work`
- Create: `.env.example`
- Create: `.gitignore`
- Create: `packages/contracts/package.json`
- Create: `packages/contracts/tsconfig.json`
- Create: `packages/contracts/src/index.ts`
- Create: `packages/contracts/src/user.ts`
- Create: `packages/contracts/src/session.ts`
- Create: `packages/eslint-config/package.json`
- Create: `packages/eslint-config/index.js`
- Create: `packages/ui/package.json`
- Create: `packages/ui/tsconfig.json`
- Create: `packages/ui/src/index.ts`

- [ ] **Step 1: Initialize Turborepo root**

```json
// turbo.json
{
  "$schema": "https://turbo.build/schema.json",
  "globalDependencies": [".env"],
  "tasks": {
    "build": {
      "dependsOn": ["^build"],
      "outputs": [".next/**", "dist/**"]
    },
    "dev": {
      "cache": false,
      "persistent": true
    },
    "lint": {
      "dependsOn": ["^build"]
    },
    "test": {
      "dependsOn": ["^build"]
    }
  }
}
```

```json
// package.json
{
  "name": "gpass",
  "private": true,
  "workspaces": ["apps/*", "packages/*"],
  "scripts": {
    "dev": "turbo dev",
    "build": "turbo build",
    "lint": "turbo lint",
    "test": "turbo test"
  },
  "devDependencies": {
    "turbo": "^2.4.0"
  },
  "packageManager": "pnpm@9.15.0"
}
```

- [ ] **Step 2: Create Go workspace**

```
// go.work
go 1.22

use (
	./apps/bff
)
```

- [ ] **Step 3: Create .env.example**

```bash
# .env.example

# Keycloak
KEYCLOAK_URL=http://localhost:8080
KEYCLOAK_REALM=garudapass
KEYCLOAK_CLIENT_ID=bff-client
KEYCLOAK_JWKS_URI=http://localhost:8080/realms/garudapass/protocol/openid-connect/certs

# BFF
BFF_PORT=4000
BFF_SESSION_SECRET=change-me-to-random-64-chars
BFF_COOKIE_DOMAIN=localhost
BFF_REDIRECT_URI=http://localhost:4000/auth/callback
BFF_FRONTEND_URL=http://localhost:3000

# Redis
REDIS_URL=redis://localhost:6379

# PostgreSQL
DATABASE_URL=postgres://garudapass:garudapass@localhost:5432/garudapass?sslmode=disable

# Kafka
KAFKA_BROKERS=localhost:9092

# Kong
KONG_ADMIN_URL=http://localhost:8001
```

- [ ] **Step 4: Create .gitignore**

```gitignore
# .gitignore
node_modules/
.next/
dist/
.turbo/
.env
*.log
.superpowers/
```

- [ ] **Step 5: Create shared contracts package**

```json
// packages/contracts/package.json
{
  "name": "@gpass/contracts",
  "version": "0.1.0",
  "private": true,
  "main": "./src/index.ts",
  "types": "./src/index.ts",
  "scripts": {
    "lint": "eslint src/",
    "test": "echo 'no tests yet'"
  },
  "devDependencies": {
    "typescript": "^5.7.0"
  }
}
```

```json
// packages/contracts/tsconfig.json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "declaration": true,
    "outDir": "dist",
    "rootDir": "src"
  },
  "include": ["src"]
}
```

```typescript
// packages/contracts/src/user.ts
export interface GarudaPassUser {
  id: string;
  nik_masked: string; // e.g., "************3456"
  name: string;
  email: string;
  phone: string;
  verified: boolean;
  auth_level: 0 | 1 | 2 | 3 | 4;
  created_at: string;
}
```

```typescript
// packages/contracts/src/session.ts
export interface SessionInfo {
  user: {
    id: string;
    name: string;
    email: string;
    verified: boolean;
    auth_level: number;
  } | null;
  authenticated: boolean;
  csrf_token: string;
}
```

```typescript
// packages/contracts/src/index.ts
export type { GarudaPassUser } from "./user";
export type { SessionInfo } from "./session";
```

- [ ] **Step 6: Create Makefile**

```makefile
# Makefile
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
```

- [ ] **Step 7: Verify monorepo structure**

Run: `ls -la && cat turbo.json && cat go.work`
Expected: All files present, valid JSON/Go syntax.

- [ ] **Step 8: Commit**

```bash
git add turbo.json package.json go.work .env.example .gitignore Makefile packages/
git commit -m "feat: scaffold Turborepo monorepo with Go workspace and shared contracts"
```

---

## Task 2: Docker Compose Local Development Environment

**Files:**
- Create: `docker-compose.yml`
- Create: `infrastructure/kafka/topics.sh`
- Create: `tools/scripts/setup.sh`
- Create: `tools/scripts/generate-keys.sh`

- [ ] **Step 1: Create Docker Compose file**

```yaml
# docker-compose.yml
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: garudapass
      POSTGRES_PASSWORD: garudapass
      POSTGRES_DB: garudapass
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U garudapass"]
      interval: 5s
      timeout: 3s
      retries: 5

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 3s
      retries: 5

  redpanda:
    image: redpandadata/redpanda:v24.3.1
    command:
      - redpanda start
      - --smp 1
      - --memory 512M
      - --overprovisioned
      - --node-id 0
      - --kafka-addr internal://0.0.0.0:9092,external://0.0.0.0:19092
      - --advertise-kafka-addr internal://redpanda:9092,external://localhost:19092
    ports:
      - "19092:19092"
      - "9644:9644"
    healthcheck:
      test: ["CMD", "rpk", "cluster", "health"]
      interval: 10s
      timeout: 5s
      retries: 5

  keycloak:
    image: quay.io/keycloak/keycloak:26.0
    environment:
      KC_DB: postgres
      KC_DB_URL: jdbc:postgresql://postgres:5432/garudapass
      KC_DB_USERNAME: garudapass
      KC_DB_PASSWORD: garudapass
      KC_HOSTNAME: localhost
      KC_HTTP_ENABLED: "true"
      KC_HEALTH_ENABLED: "true"
      KEYCLOAK_ADMIN: admin
      KEYCLOAK_ADMIN_PASSWORD: admin
    command: start-dev --import-realm
    ports:
      - "8080:8080"
    volumes:
      - ./infrastructure/keycloak/realm-export.json:/opt/keycloak/data/import/realm-export.json
    depends_on:
      postgres:
        condition: service_healthy

  kong:
    image: kong:3.9
    environment:
      KONG_DATABASE: "off"
      KONG_DECLARATIVE_CONFIG: /etc/kong/kong.yml
      KONG_PROXY_LISTEN: "0.0.0.0:8000"
      KONG_ADMIN_LISTEN: "0.0.0.0:8001"
      KONG_LOG_LEVEL: info
    ports:
      - "8000:8000"
      - "8001:8001"
    volumes:
      - ./infrastructure/kong/kong.yml:/etc/kong/kong.yml

volumes:
  pgdata:
```

- [ ] **Step 2: Create Kafka topic setup script**

```bash
#!/usr/bin/env bash
# infrastructure/kafka/topics.sh
set -euo pipefail

BROKER="${KAFKA_BROKERS:-localhost:19092}"

topics=(
  "audit.auth"
  "audit.consent"
  "audit.signing"
  "audit.corporate"
  "events.user.lifecycle"
  "events.entity.lifecycle"
)

for topic in "${topics[@]}"; do
  rpk topic create "$topic" \
    --brokers "$BROKER" \
    --partitions 3 \
    --replicas 1 \
    2>/dev/null || echo "Topic $topic already exists"
done

echo "All Kafka topics created."
```

- [ ] **Step 3: Create key generation script**

```bash
#!/usr/bin/env bash
# tools/scripts/generate-keys.sh
set -euo pipefail

KEY_DIR="./keys"
mkdir -p "$KEY_DIR"

echo "Generating EC P-256 key pair for BFF OIDC client..."
openssl ecparam -genkey -name prime256v1 -noout -out "$KEY_DIR/bff-private.pem"
openssl ec -in "$KEY_DIR/bff-private.pem" -pubout -out "$KEY_DIR/bff-public.pem"

# Generate JWKS format for Keycloak registration
openssl ec -in "$KEY_DIR/bff-private.pem" -pubout -outform DER 2>/dev/null | \
  openssl base64 -A > "$KEY_DIR/bff-public.b64"

echo "Keys generated in $KEY_DIR/"
echo "  Private: $KEY_DIR/bff-private.pem"
echo "  Public:  $KEY_DIR/bff-public.pem"
echo ""
echo "Add $KEY_DIR/ to .gitignore (NEVER commit private keys)"
```

- [ ] **Step 4: Create setup script**

```bash
#!/usr/bin/env bash
# tools/scripts/setup.sh
set -euo pipefail

echo "=== GarudaPass Development Setup ==="

# Check prerequisites
for cmd in docker pnpm go openssl; do
  if ! command -v "$cmd" &> /dev/null; then
    echo "ERROR: $cmd is required but not installed."
    exit 1
  fi
done

# Copy env if not exists
if [ ! -f .env ]; then
  cp .env.example .env
  echo "Created .env from .env.example"
fi

# Install dependencies
echo "Installing Node.js dependencies..."
pnpm install

echo "Installing Go dependencies..."
cd apps/bff && go mod download && cd ../..

# Generate keys
echo "Generating OIDC client keys..."
./tools/scripts/generate-keys.sh

# Start infrastructure
echo "Starting Docker services..."
docker compose up -d

# Wait for services
echo "Waiting for services to be healthy..."
sleep 10

# Create Kafka topics
echo "Creating Kafka topics..."
chmod +x infrastructure/kafka/topics.sh
KAFKA_BROKERS=localhost:19092 ./infrastructure/kafka/topics.sh

echo ""
echo "=== Setup complete! ==="
echo "Keycloak Admin: http://localhost:8080 (admin/admin)"
echo "Kong Proxy:     http://localhost:8000"
echo "PostgreSQL:     localhost:5432"
echo "Redis:          localhost:6379"
echo "Kafka:          localhost:19092"
```

- [ ] **Step 5: Add keys/ to .gitignore**

Append to `.gitignore`:
```
keys/
```

- [ ] **Step 6: Verify Docker Compose starts**

Run: `docker compose config --quiet && echo "Valid"`
Expected: No errors, `Valid` printed.

- [ ] **Step 7: Commit**

```bash
git add docker-compose.yml infrastructure/kafka/ tools/scripts/ .gitignore
git commit -m "feat: add Docker Compose dev environment with Keycloak, PostgreSQL, Redis, Kafka, Kong"
```

---

## Task 3: Keycloak Realm Configuration (FAPI 2.0)

**Files:**
- Create: `infrastructure/keycloak/realm-export.json`
- Create: `infrastructure/keycloak/themes/garudapass/login/theme.properties`

- [ ] **Step 1: Create GarudaPass realm export**

```json
// infrastructure/keycloak/realm-export.json
{
  "realm": "garudapass",
  "enabled": true,
  "registrationAllowed": false,
  "loginWithEmailAllowed": true,
  "duplicateEmailsAllowed": false,
  "sslRequired": "none",
  "defaultSignatureAlgorithm": "ES256",
  "accessTokenLifespan": 600,
  "ssoSessionIdleTimeout": 1800,
  "ssoSessionMaxLifespan": 36000,
  "accessCodeLifespan": 60,
  "oauth2DeviceCodeLifespan": 600,
  "attributes": {
    "parSupported": "true",
    "requirePushedAuthorizationRequests": "true",
    "dpopEnforced": "true"
  },
  "roles": {
    "realm": [
      { "name": "gpass_user", "description": "Standard GarudaPass user" },
      { "name": "gpass_admin", "description": "GarudaPass platform admin" },
      { "name": "gpass_developer", "description": "Developer with API access" }
    ]
  },
  "clients": [
    {
      "clientId": "bff-client",
      "name": "GarudaPass Web BFF",
      "enabled": true,
      "protocol": "openid-connect",
      "publicClient": false,
      "clientAuthenticatorType": "client-jwt",
      "standardFlowEnabled": true,
      "directAccessGrantsEnabled": false,
      "serviceAccountsEnabled": false,
      "redirectUris": [
        "http://localhost:4000/auth/callback"
      ],
      "webOrigins": [
        "http://localhost:3000"
      ],
      "attributes": {
        "pkce.code.challenge.method": "S256",
        "token.endpoint.auth.signing.alg": "ES256",
        "use.refresh.tokens": "true",
        "client_credentials.use_refresh_token": "false",
        "require.pushed.authorization.requests": "true",
        "id.token.signed.response.alg": "ES256",
        "access.token.signed.response.alg": "ES256"
      },
      "defaultClientScopes": [
        "openid",
        "profile",
        "email"
      ],
      "optionalClientScopes": [
        "garudainfo",
        "authinfo"
      ]
    },
    {
      "clientId": "sandbox-test-client",
      "name": "Sandbox Test Client",
      "enabled": true,
      "protocol": "openid-connect",
      "publicClient": false,
      "clientAuthenticatorType": "client-secret",
      "secret": "sandbox-test-secret-change-in-production",
      "standardFlowEnabled": true,
      "directAccessGrantsEnabled": true,
      "redirectUris": ["http://localhost:*"],
      "webOrigins": ["*"],
      "attributes": {
        "pkce.code.challenge.method": "S256"
      },
      "defaultClientScopes": ["openid", "profile", "email"]
    }
  ],
  "clientScopes": [
    {
      "name": "garudainfo",
      "description": "Access to GarudaInfo verified personal data",
      "protocol": "openid-connect",
      "attributes": {
        "consent.screen.text": "Access your verified personal data (name, NIK, address)"
      },
      "protocolMappers": [
        {
          "name": "nik-masked",
          "protocol": "openid-connect",
          "protocolMapper": "oidc-usermodel-attribute-mapper",
          "config": {
            "user.attribute": "nik_masked",
            "claim.name": "nik_masked",
            "jsonType.label": "String",
            "id.token.claim": "false",
            "access.token.claim": "true",
            "userinfo.token.claim": "true"
          }
        }
      ]
    },
    {
      "name": "authinfo",
      "description": "Corporate authorization info (role, entity, access rights)",
      "protocol": "openid-connect",
      "attributes": {
        "consent.screen.text": "Access your corporate role and authorization information"
      }
    }
  ],
  "users": [
    {
      "username": "testuser",
      "email": "test@garudapass.id",
      "enabled": true,
      "firstName": "Test",
      "lastName": "User",
      "credentials": [
        {
          "type": "password",
          "value": "testpassword123",
          "temporary": false
        }
      ],
      "realmRoles": ["gpass_user"],
      "attributes": {
        "nik_masked": "************1234",
        "verified": "true",
        "auth_level": "2"
      }
    },
    {
      "username": "admin",
      "email": "admin@garudapass.id",
      "enabled": true,
      "firstName": "Admin",
      "lastName": "GarudaPass",
      "credentials": [
        {
          "type": "password",
          "value": "adminpassword123",
          "temporary": false
        }
      ],
      "realmRoles": ["gpass_admin", "gpass_user"]
    }
  ]
}
```

- [ ] **Step 2: Create GarudaPass login theme**

```properties
# infrastructure/keycloak/themes/garudapass/login/theme.properties
parent=keycloak.v2
import=common/keycloak
styles=css/login.css
```

- [ ] **Step 3: Verify realm imports on Keycloak start**

Run: `docker compose up -d keycloak && sleep 15 && curl -s http://localhost:8080/realms/garudapass/.well-known/openid-configuration | python3 -m json.tool | head -20`

Expected: JSON response with `issuer: "http://localhost:8080/realms/garudapass"` and `pushed_authorization_request_endpoint` present.

- [ ] **Step 4: Commit**

```bash
git add infrastructure/keycloak/
git commit -m "feat: add Keycloak realm config with FAPI 2.0 (PAR, DPoP, PKCE S256, ES256 signing)"
```

---

## Task 4: Kong API Gateway Configuration

**Files:**
- Create: `infrastructure/kong/kong.yml`

- [ ] **Step 1: Create declarative Kong config**

```yaml
# infrastructure/kong/kong.yml
_format_version: "3.0"

services:
  - name: bff-service
    url: http://host.docker.internal:4000
    routes:
      - name: bff-auth-routes
        paths:
          - /auth
        strip_path: false
      - name: bff-api-routes
        paths:
          - /api
        strip_path: false

  - name: keycloak-service
    url: http://keycloak:8080
    routes:
      - name: keycloak-well-known
        paths:
          - /realms
        strip_path: false

plugins:
  - name: rate-limiting
    config:
      minute: 60
      policy: local
    route: bff-auth-routes

  - name: rate-limiting
    config:
      minute: 300
      policy: local
    route: bff-api-routes

  - name: cors
    config:
      origins:
        - http://localhost:3000
        - http://localhost:3001
      methods:
        - GET
        - POST
        - PUT
        - DELETE
        - OPTIONS
      headers:
        - Content-Type
        - Authorization
        - X-CSRF-Token
      credentials: true
      max_age: 3600

  - name: request-size-limiting
    config:
      allowed_payload_size: 10
      size_unit: megabytes
```

- [ ] **Step 2: Verify Kong config loads**

Run: `docker compose up -d kong && sleep 5 && curl -s http://localhost:8001/status | python3 -m json.tool | head -10`

Expected: JSON with `database.reachable: true` (or similar status for DB-less mode).

- [ ] **Step 3: Commit**

```bash
git add infrastructure/kong/
git commit -m "feat: add Kong API gateway config with rate limiting, CORS, and request size limits"
```

---

## Task 5: Go BFF — Project Setup and Config

**Files:**
- Create: `apps/bff/go.mod`
- Create: `apps/bff/config/config.go`
- Create: `apps/bff/config/config_test.go`

- [ ] **Step 1: Initialize Go module**

Run: `cd apps/bff && go mod init github.com/garudapass/gpass/apps/bff`

- [ ] **Step 2: Write the config test**

```go
// apps/bff/config/config_test.go
package config_test

import (
	"os"
	"testing"

	"github.com/garudapass/gpass/apps/bff/config"
)

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("BFF_PORT", "4000")
	os.Setenv("KEYCLOAK_URL", "http://localhost:8080")
	os.Setenv("KEYCLOAK_REALM", "garudapass")
	os.Setenv("KEYCLOAK_CLIENT_ID", "bff-client")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
	os.Setenv("BFF_SESSION_SECRET", "test-secret-must-be-at-least-32-characters-long-ok")
	os.Setenv("BFF_FRONTEND_URL", "http://localhost:3000")
	os.Setenv("BFF_REDIRECT_URI", "http://localhost:4000/auth/callback")
	defer func() {
		for _, k := range []string{"BFF_PORT", "KEYCLOAK_URL", "KEYCLOAK_REALM", "KEYCLOAK_CLIENT_ID", "REDIS_URL", "BFF_SESSION_SECRET", "BFF_FRONTEND_URL", "BFF_REDIRECT_URI"} {
			os.Unsetenv(k)
		}
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "4000" {
		t.Errorf("expected port 4000, got %s", cfg.Port)
	}
	if cfg.KeycloakURL != "http://localhost:8080" {
		t.Errorf("expected keycloak URL http://localhost:8080, got %s", cfg.KeycloakURL)
	}
	if cfg.IssuerURL() != "http://localhost:8080/realms/garudapass" {
		t.Errorf("expected issuer URL with realm, got %s", cfg.IssuerURL())
	}
}

func TestLoadMissingRequired(t *testing.T) {
	os.Clearenv()

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing required env vars")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd apps/bff && go test ./config/... -v`
Expected: FAIL — package `config` does not exist yet.

- [ ] **Step 4: Write config implementation**

```go
// apps/bff/config/config.go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port           string
	KeycloakURL    string
	KeycloakRealm  string
	ClientID       string
	RedisURL       string
	SessionSecret  string
	FrontendURL    string
	RedirectURI    string
	CookieDomain   string
}

func (c *Config) IssuerURL() string {
	return fmt.Sprintf("%s/realms/%s", c.KeycloakURL, c.KeycloakRealm)
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:          getEnv("BFF_PORT", "4000"),
		KeycloakURL:   os.Getenv("KEYCLOAK_URL"),
		KeycloakRealm: os.Getenv("KEYCLOAK_REALM"),
		ClientID:      os.Getenv("KEYCLOAK_CLIENT_ID"),
		RedisURL:      getEnv("REDIS_URL", "redis://localhost:6379"),
		SessionSecret: os.Getenv("BFF_SESSION_SECRET"),
		FrontendURL:   os.Getenv("BFF_FRONTEND_URL"),
		RedirectURI:   os.Getenv("BFF_REDIRECT_URI"),
		CookieDomain:  getEnv("BFF_COOKIE_DOMAIN", "localhost"),
	}

	required := map[string]string{
		"KEYCLOAK_URL":       cfg.KeycloakURL,
		"KEYCLOAK_REALM":     cfg.KeycloakRealm,
		"KEYCLOAK_CLIENT_ID": cfg.ClientID,
		"BFF_SESSION_SECRET": cfg.SessionSecret,
		"BFF_FRONTEND_URL":   cfg.FrontendURL,
		"BFF_REDIRECT_URI":   cfg.RedirectURI,
	}

	for name, val := range required {
		if val == "" {
			return nil, fmt.Errorf("required environment variable %s is not set", name)
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd apps/bff && go test ./config/... -v`
Expected: PASS — both tests pass.

- [ ] **Step 6: Commit**

```bash
git add apps/bff/go.mod apps/bff/config/
git commit -m "feat(bff): add environment-based configuration with validation"
```

---

## Task 6: Go BFF — Redis Session Store

**Files:**
- Create: `apps/bff/session/store.go`
- Create: `apps/bff/session/store_test.go`

- [ ] **Step 1: Add redis dependency**

Run: `cd apps/bff && go get github.com/redis/go-redis/v9`

- [ ] **Step 2: Write session store test**

```go
// apps/bff/session/store_test.go
package session_test

import (
	"context"
	"testing"
	"time"

	"github.com/garudapass/gpass/apps/bff/session"
)

func TestInMemoryStore(t *testing.T) {
	store := session.NewInMemoryStore()
	ctx := context.Background()

	data := &session.Data{
		UserID:       "user-123",
		AccessToken:  "at-token",
		RefreshToken: "rt-token",
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	}

	sid, err := store.Create(ctx, data, 30*time.Minute)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if sid == "" {
		t.Fatal("expected non-empty session ID")
	}

	got, err := store.Get(ctx, sid)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.UserID != "user-123" {
		t.Errorf("expected user-123, got %s", got.UserID)
	}
	if got.AccessToken != "at-token" {
		t.Errorf("expected at-token, got %s", got.AccessToken)
	}

	err = store.Delete(ctx, sid)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.Get(ctx, sid)
	if err != session.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd apps/bff && go test ./session/... -v`
Expected: FAIL — package does not exist.

- [ ] **Step 4: Write session store implementation**

```go
// apps/bff/session/store.go
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrSessionNotFound = errors.New("session not found")

type Data struct {
	UserID       string    `json:"user_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	IDToken      string    `json:"id_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	CSRFToken    string    `json:"csrf_token"`
}

type Store interface {
	Create(ctx context.Context, data *Data, ttl time.Duration) (string, error)
	Get(ctx context.Context, sessionID string) (*Data, error)
	Update(ctx context.Context, sessionID string, data *Data, ttl time.Duration) error
	Delete(ctx context.Context, sessionID string) error
}

func generateID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// --- Redis implementation ---

type RedisStore struct {
	client *redis.Client
	prefix string
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client, prefix: "gpass:session:"}
}

func (s *RedisStore) Create(ctx context.Context, data *Data, ttl time.Duration) (string, error) {
	sid, err := generateID()
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	if err := s.client.Set(ctx, s.prefix+sid, b, ttl).Err(); err != nil {
		return "", err
	}
	return sid, nil
}

func (s *RedisStore) Get(ctx context.Context, sessionID string) (*Data, error) {
	b, err := s.client.Get(ctx, s.prefix+sessionID).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	var data Data
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (s *RedisStore) Update(ctx context.Context, sessionID string, data *Data, ttl time.Duration) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, s.prefix+sessionID, b, ttl).Err()
}

func (s *RedisStore) Delete(ctx context.Context, sessionID string) error {
	return s.client.Del(ctx, s.prefix+sessionID).Err()
}

// --- In-memory implementation (for testing) ---

type InMemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]*Data
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{sessions: make(map[string]*Data)}
}

func (s *InMemoryStore) Create(_ context.Context, data *Data, _ time.Duration) (string, error) {
	sid, err := generateID()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *data
	s.sessions[sid] = &cp
	return sid, nil
}

func (s *InMemoryStore) Get(_ context.Context, sessionID string) (*Data, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	cp := *d
	return &cp, nil
}

func (s *InMemoryStore) Update(_ context.Context, sessionID string, data *Data, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *data
	s.sessions[sessionID] = &cp
	return nil
}

func (s *InMemoryStore) Delete(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd apps/bff && go test ./session/... -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add apps/bff/session/ apps/bff/go.mod apps/bff/go.sum
git commit -m "feat(bff): add Redis-backed session store with in-memory fallback for testing"
```

---

## Task 7: Go BFF — Auth Handlers (Login/Callback/Logout)

**Files:**
- Create: `apps/bff/handler/auth.go`
- Create: `apps/bff/handler/auth_test.go`
- Create: `apps/bff/handler/session.go`
- Create: `apps/bff/handler/session_test.go`

- [ ] **Step 1: Write auth handler test**

```go
// apps/bff/handler/auth_test.go
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/apps/bff/handler"
	"github.com/garudapass/gpass/apps/bff/session"
)

func TestLoginRedirect(t *testing.T) {
	store := session.NewInMemoryStore()
	h := handler.NewAuthHandler(handler.AuthConfig{
		IssuerURL:   "http://keycloak:8080/realms/garudapass",
		ClientID:    "bff-client",
		RedirectURI: "http://localhost:4000/auth/callback",
		FrontendURL: "http://localhost:3000",
	}, store)

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}

	loc := w.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header")
	}

	// Should redirect to Keycloak PAR or authorize endpoint
	// with PKCE code_challenge and state
	if len(loc) < 50 {
		t.Errorf("redirect URL too short, likely malformed: %s", loc)
	}
}

func TestLogout(t *testing.T) {
	store := session.NewInMemoryStore()
	h := handler.NewAuthHandler(handler.AuthConfig{
		IssuerURL:   "http://keycloak:8080/realms/garudapass",
		ClientID:    "bff-client",
		RedirectURI: "http://localhost:4000/auth/callback",
		FrontendURL: "http://localhost:3000",
	}, store)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	w := httptest.NewRecorder()

	h.Logout(w, req)

	// Should clear cookie and redirect to frontend
	if w.Code != http.StatusFound {
		t.Errorf("expected 302, got %d", w.Code)
	}
	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "gpass_session" && c.MaxAge < 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected gpass_session cookie to be cleared")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/bff && go test ./handler/... -v`
Expected: FAIL — handler package does not exist.

- [ ] **Step 3: Write auth handler implementation**

```go
// apps/bff/handler/auth.go
package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/garudapass/gpass/apps/bff/session"
)

const (
	cookieName     = "gpass_session"
	sessionTTL     = 30 * time.Minute
	stateCookieMax = 300 // 5 minutes
)

type AuthConfig struct {
	IssuerURL   string
	ClientID    string
	RedirectURI string
	FrontendURL string
}

type AuthHandler struct {
	cfg   AuthConfig
	store session.Store
}

func NewAuthHandler(cfg AuthConfig, store session.Store) *AuthHandler {
	return &AuthHandler{cfg: cfg, store: store}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	state, err := randomString(32)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	verifier, err := randomString(64)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	challenge := computeCodeChallenge(verifier)

	// Store verifier in a short-lived cookie (encrypted in production)
	http.SetCookie(w, &http.Cookie{
		Name:     "gpass_pkce",
		Value:    verifier,
		Path:     "/auth",
		MaxAge:   stateCookieMax,
		HttpOnly: true,
		Secure:   false, // true in production
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "gpass_state",
		Value:    state,
		Path:     "/auth",
		MaxAge:   stateCookieMax,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})

	authURL := fmt.Sprintf("%s/protocol/openid-connect/auth?"+
		"response_type=code"+
		"&client_id=%s"+
		"&redirect_uri=%s"+
		"&scope=openid+profile+email"+
		"&state=%s"+
		"&code_challenge=%s"+
		"&code_challenge_method=S256",
		h.cfg.IssuerURL,
		url.QueryEscape(h.cfg.ClientID),
		url.QueryEscape(h.cfg.RedirectURI),
		url.QueryEscape(state),
		url.QueryEscape(challenge),
	)

	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	// In a full implementation, this would:
	// 1. Validate state cookie matches state param
	// 2. Exchange authorization code for tokens using PKCE verifier
	// 3. Store tokens in Redis session
	// 4. Set session cookie
	// 5. Redirect to frontend
	//
	// This is a placeholder that will be completed when we add the OIDC client in Task 8+
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	// TODO: Token exchange will be implemented in Plan 2 with full OIDC client
	// For now, create a stub session
	sess := &session.Data{
		UserID:    "stub-user",
		CSRFToken: mustRandomString(32),
		ExpiresAt: time.Now().Add(sessionTTL),
	}

	sid, err := h.store.Create(context.Background(), sess, sessionTTL)
	if err != nil {
		http.Error(w, "session creation failed", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sid,
		Path:     "/",
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   false, // true in production
		SameSite: http.SameSiteStrictMode,
	})

	clearAuthCookies(w)
	http.Redirect(w, r, h.cfg.FrontendURL, http.StatusFound)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(cookieName)
	if err == nil && cookie.Value != "" {
		_ = h.store.Delete(context.Background(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(w, r, h.cfg.FrontendURL, http.StatusFound)
}

func computeCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b)[:n], nil
}

func mustRandomString(n int) string {
	s, err := randomString(n)
	if err != nil {
		panic(err)
	}
	return s
}

func clearAuthCookies(w http.ResponseWriter) {
	for _, name := range []string{"gpass_pkce", "gpass_state"} {
		http.SetCookie(w, &http.Cookie{
			Name:   name,
			Value:  "",
			Path:   "/auth",
			MaxAge: -1,
		})
	}
}
```

- [ ] **Step 4: Write session info handler**

```go
// apps/bff/handler/session.go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/garudapass/gpass/apps/bff/session"
)

type SessionHandler struct {
	store session.Store
}

func NewSessionHandler(store session.Store) *SessionHandler {
	return &SessionHandler{store: store}
}

type SessionResponse struct {
	Authenticated bool        `json:"authenticated"`
	User          *UserInfo   `json:"user"`
	CSRFToken     string      `json:"csrf_token,omitempty"`
}

type UserInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Verified bool   `json:"verified"`
}

func (h *SessionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		writeJSON(w, http.StatusOK, SessionResponse{Authenticated: false})
		return
	}

	data, err := h.store.Get(r.Context(), cookie.Value)
	if err != nil {
		writeJSON(w, http.StatusOK, SessionResponse{Authenticated: false})
		return
	}

	writeJSON(w, http.StatusOK, SessionResponse{
		Authenticated: true,
		User: &UserInfo{
			ID: data.UserID,
		},
		CSRFToken: data.CSRFToken,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 5: Write session handler test**

```go
// apps/bff/handler/session_test.go
package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/apps/bff/handler"
	"github.com/garudapass/gpass/apps/bff/session"
)

func TestGetSessionUnauthenticated(t *testing.T) {
	store := session.NewInMemoryStore()
	h := handler.NewSessionHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	w := httptest.NewRecorder()

	h.GetSession(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp handler.SessionResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Authenticated {
		t.Error("expected authenticated=false")
	}
	if resp.User != nil {
		t.Error("expected user=nil")
	}
}

func TestGetSessionAuthenticated(t *testing.T) {
	store := session.NewInMemoryStore()
	h := handler.NewSessionHandler(store)

	sid, _ := store.Create(context.Background(), &session.Data{
		UserID:    "user-456",
		CSRFToken: "csrf-token-123",
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}, 30*time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/auth/session", nil)
	req.AddCookie(&http.Cookie{Name: "gpass_session", Value: sid})
	w := httptest.NewRecorder()

	h.GetSession(w, req)

	var resp handler.SessionResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if !resp.Authenticated {
		t.Error("expected authenticated=true")
	}
	if resp.User.ID != "user-456" {
		t.Errorf("expected user-456, got %s", resp.User.ID)
	}
	if resp.CSRFToken != "csrf-token-123" {
		t.Errorf("expected csrf-token-123, got %s", resp.CSRFToken)
	}
}
```

- [ ] **Step 6: Run all handler tests**

Run: `cd apps/bff && go test ./handler/... -v`
Expected: PASS — all 4 tests pass.

- [ ] **Step 7: Commit**

```bash
git add apps/bff/handler/
git commit -m "feat(bff): add auth handlers (login/callback/logout) and session endpoint"
```

---

## Task 8: Go BFF — CSRF Middleware

**Files:**
- Create: `apps/bff/middleware/csrf.go`
- Create: `apps/bff/middleware/csrf_test.go`

- [ ] **Step 1: Write CSRF middleware test**

```go
// apps/bff/middleware/csrf_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/apps/bff/middleware"
)

func TestCSRFRejectsPostWithoutToken(t *testing.T) {
	handler := middleware.CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestCSRFAllowsGetWithoutToken(t *testing.T) {
	handler := middleware.CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCSRFAllowsPostWithValidToken(t *testing.T) {
	handler := middleware.CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	token := "valid-csrf-token"
	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req.Header.Set("X-CSRF-Token", token)
	req.AddCookie(&http.Cookie{Name: "gpass_csrf", Value: token})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd apps/bff && go test ./middleware/... -v`
Expected: FAIL.

- [ ] **Step 3: Write CSRF middleware implementation**

```go
// apps/bff/middleware/csrf.go
package middleware

import (
	"net/http"
)

// CSRF implements double-submit cookie pattern.
// The CSRF token is set as a cookie and must be sent back in the X-CSRF-Token header.
// Safe methods (GET, HEAD, OPTIONS) are exempt.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		headerToken := r.Header.Get("X-CSRF-Token")
		if headerToken == "" {
			http.Error(w, "missing CSRF token", http.StatusForbidden)
			return
		}

		cookie, err := r.Cookie("gpass_csrf")
		if err != nil || cookie.Value == "" {
			http.Error(w, "missing CSRF cookie", http.StatusForbidden)
			return
		}

		if headerToken != cookie.Value {
			http.Error(w, "CSRF token mismatch", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}
```

- [ ] **Step 4: Run tests**

Run: `cd apps/bff && go test ./middleware/... -v`
Expected: PASS — all 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add apps/bff/middleware/
git commit -m "feat(bff): add double-submit cookie CSRF middleware"
```

---

## Task 9: Go BFF — Main Server

**Files:**
- Create: `apps/bff/main.go`
- Create: `apps/bff/Dockerfile`

- [ ] **Step 1: Write main.go server**

```go
// apps/bff/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/garudapass/gpass/apps/bff/config"
	"github.com/garudapass/gpass/apps/bff/handler"
	"github.com/garudapass/gpass/apps/bff/middleware"
	"github.com/garudapass/gpass/apps/bff/session"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Redis
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis URL: %v", err)
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Printf("WARNING: Redis not reachable: %v (falling back to in-memory sessions)", err)
	}

	store := session.NewRedisStore(rdb)

	authHandler := handler.NewAuthHandler(handler.AuthConfig{
		IssuerURL:   cfg.IssuerURL(),
		ClientID:    cfg.ClientID,
		RedirectURI: cfg.RedirectURI,
		FrontendURL: cfg.FrontendURL,
	}, store)
	sessionHandler := handler.NewSessionHandler(store)

	mux := http.NewServeMux()

	// Auth routes (no CSRF — these initiate/complete OAuth flows)
	mux.HandleFunc("GET /auth/login", authHandler.Login)
	mux.HandleFunc("GET /auth/callback", authHandler.Callback)
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /auth/session", sessionHandler.GetSession)

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// API routes (with CSRF protection)
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/v1/me", sessionHandler.GetSession)
	mux.Handle("/api/", middleware.CSRF(apiMux))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("BFF listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)
	log.Println("BFF shut down")
}
```

- [ ] **Step 2: Create Dockerfile**

```dockerfile
# apps/bff/Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bff .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /bff /bff
EXPOSE 4000
CMD ["/bff"]
```

- [ ] **Step 3: Verify it compiles**

Run: `cd apps/bff && go build -o /dev/null .`
Expected: No errors.

- [ ] **Step 4: Run all BFF tests**

Run: `cd apps/bff && go test ./... -v`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
git add apps/bff/main.go apps/bff/Dockerfile
git commit -m "feat(bff): add main server with health check, auth routes, CSRF-protected API routes"
```

---

## Task 10: Next.js Citizen Portal Scaffold

**Files:**
- Create: `apps/citizen-portal/package.json`
- Create: `apps/citizen-portal/next.config.js`
- Create: `apps/citizen-portal/tsconfig.json`
- Create: `apps/citizen-portal/app/layout.tsx`
- Create: `apps/citizen-portal/app/page.tsx`
- Create: `apps/citizen-portal/app/login/page.tsx`
- Create: `apps/citizen-portal/lib/api.ts`
- Create: `apps/citizen-portal/components/auth-provider.tsx`

- [ ] **Step 1: Create package.json**

```json
// apps/citizen-portal/package.json
{
  "name": "@gpass/citizen-portal",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "next dev --port 3000",
    "build": "next build",
    "start": "next start",
    "lint": "next lint",
    "test": "echo 'no tests yet'"
  },
  "dependencies": {
    "next": "^14.2.0",
    "react": "^18.3.0",
    "react-dom": "^18.3.0",
    "@gpass/contracts": "workspace:*"
  },
  "devDependencies": {
    "@types/node": "^22.0.0",
    "@types/react": "^18.3.0",
    "typescript": "^5.7.0"
  }
}
```

- [ ] **Step 2: Create next.config.js**

```javascript
// apps/citizen-portal/next.config.js
/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      {
        source: "/auth/:path*",
        destination: `${process.env.BFF_URL || "http://localhost:4000"}/auth/:path*`,
      },
      {
        source: "/api/:path*",
        destination: `${process.env.BFF_URL || "http://localhost:4000"}/api/:path*`,
      },
    ];
  },
};

module.exports = nextConfig;
```

- [ ] **Step 3: Create tsconfig.json**

```json
// apps/citizen-portal/tsconfig.json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["dom", "dom.iterable", "esnext"],
    "allowJs": true,
    "skipLibCheck": true,
    "strict": true,
    "noEmit": true,
    "esModuleInterop": true,
    "module": "esnext",
    "moduleResolution": "bundler",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "jsx": "preserve",
    "incremental": true,
    "plugins": [{ "name": "next" }],
    "paths": { "@/*": ["./*"] }
  },
  "include": ["next-env.d.ts", "**/*.ts", "**/*.tsx"],
  "exclude": ["node_modules"]
}
```

- [ ] **Step 4: Create API client**

```typescript
// apps/citizen-portal/lib/api.ts
import type { SessionInfo } from "@gpass/contracts";

const BFF_BASE = "";

export async function getSession(): Promise<SessionInfo> {
  const res = await fetch(`${BFF_BASE}/auth/session`, {
    credentials: "include",
  });
  if (!res.ok) {
    return { user: null, authenticated: false, csrf_token: "" };
  }
  return res.json();
}

export function loginUrl(): string {
  return `/auth/login`;
}

export function logoutUrl(): string {
  return `/auth/logout`;
}
```

- [ ] **Step 5: Create auth provider**

```tsx
// apps/citizen-portal/components/auth-provider.tsx
"use client";

import { createContext, useContext, useEffect, useState, ReactNode } from "react";
import type { SessionInfo } from "@gpass/contracts";
import { getSession } from "@/lib/api";

const AuthContext = createContext<SessionInfo>({
  user: null,
  authenticated: false,
  csrf_token: "",
});

export function useAuth() {
  return useContext(AuthContext);
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<SessionInfo>({
    user: null,
    authenticated: false,
    csrf_token: "",
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getSession()
      .then(setSession)
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return <div style={{ padding: 40, textAlign: "center" }}>Loading...</div>;
  }

  return <AuthContext.Provider value={session}>{children}</AuthContext.Provider>;
}
```

- [ ] **Step 6: Create root layout**

```tsx
// apps/citizen-portal/app/layout.tsx
import { AuthProvider } from "@/components/auth-provider";

export const metadata = {
  title: "GarudaPass",
  description: "Indonesia's Unified Identity Platform",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="id">
      <body>
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  );
}
```

- [ ] **Step 7: Create landing page**

```tsx
// apps/citizen-portal/app/page.tsx
"use client";

import { useAuth } from "@/components/auth-provider";
import { loginUrl, logoutUrl } from "@/lib/api";

export default function Home() {
  const { authenticated, user } = useAuth();

  return (
    <main style={{ maxWidth: 600, margin: "0 auto", padding: 40 }}>
      <h1>GarudaPass</h1>
      <p>Indonesia&apos;s Unified Identity Platform</p>

      {authenticated && user ? (
        <div>
          <p>Welcome, <strong>{user.name || user.id}</strong></p>
          <form action={logoutUrl()} method="POST">
            <button type="submit">Logout</button>
          </form>
        </div>
      ) : (
        <div>
          <a href={loginUrl()}>
            <button>Login with GarudaPass</button>
          </a>
        </div>
      )}
    </main>
  );
}
```

- [ ] **Step 8: Create login page**

```tsx
// apps/citizen-portal/app/login/page.tsx
"use client";

import { useEffect } from "react";
import { loginUrl } from "@/lib/api";

export default function LoginPage() {
  useEffect(() => {
    window.location.href = loginUrl();
  }, []);

  return (
    <main style={{ padding: 40, textAlign: "center" }}>
      <p>Redirecting to GarudaPass login...</p>
    </main>
  );
}
```

- [ ] **Step 9: Install dependencies and verify build**

Run: `cd apps/citizen-portal && pnpm install && pnpm build`
Expected: Build succeeds (or lint-only check if Next.js needs runtime dependencies resolved via `pnpm install` at root).

- [ ] **Step 10: Commit**

```bash
git add apps/citizen-portal/
git commit -m "feat: add Next.js citizen portal with auth provider, login flow, and BFF proxy"
```

---

## Task 11: Admin Portal Scaffold

**Files:**
- Create: `apps/admin-portal/package.json`
- Create: `apps/admin-portal/next.config.js`
- Create: `apps/admin-portal/tsconfig.json`
- Create: `apps/admin-portal/app/layout.tsx`
- Create: `apps/admin-portal/app/page.tsx`

- [ ] **Step 1: Create admin portal (similar structure to citizen portal)**

```json
// apps/admin-portal/package.json
{
  "name": "@gpass/admin-portal",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "next dev --port 3001",
    "build": "next build",
    "start": "next start",
    "lint": "next lint",
    "test": "echo 'no tests yet'"
  },
  "dependencies": {
    "next": "^14.2.0",
    "react": "^18.3.0",
    "react-dom": "^18.3.0",
    "@gpass/contracts": "workspace:*"
  },
  "devDependencies": {
    "@types/node": "^22.0.0",
    "@types/react": "^18.3.0",
    "typescript": "^5.7.0"
  }
}
```

```javascript
// apps/admin-portal/next.config.js
/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      { source: "/auth/:path*", destination: `${process.env.BFF_URL || "http://localhost:4000"}/auth/:path*` },
      { source: "/api/:path*", destination: `${process.env.BFF_URL || "http://localhost:4000"}/api/:path*` },
    ];
  },
};
module.exports = nextConfig;
```

```json
// apps/admin-portal/tsconfig.json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["dom", "dom.iterable", "esnext"],
    "allowJs": true,
    "skipLibCheck": true,
    "strict": true,
    "noEmit": true,
    "esModuleInterop": true,
    "module": "esnext",
    "moduleResolution": "bundler",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "jsx": "preserve",
    "incremental": true,
    "plugins": [{ "name": "next" }],
    "paths": { "@/*": ["./*"] }
  },
  "include": ["next-env.d.ts", "**/*.ts", "**/*.tsx"],
  "exclude": ["node_modules"]
}
```

```tsx
// apps/admin-portal/app/layout.tsx
export const metadata = {
  title: "GarudaPass Admin",
  description: "GarudaPass Administration Portal",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="id">
      <body>{children}</body>
    </html>
  );
}
```

```tsx
// apps/admin-portal/app/page.tsx
export default function AdminDashboard() {
  return (
    <main style={{ maxWidth: 800, margin: "0 auto", padding: 40 }}>
      <h1>GarudaPass Admin</h1>
      <p>Administration dashboard — coming in Plan 3.</p>
    </main>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add apps/admin-portal/
git commit -m "feat: add Next.js admin portal scaffold"
```

---

## Task 12: Kubernetes Base Manifests

**Files:**
- Create: `infrastructure/kubernetes/base/namespace.yaml`
- Create: `infrastructure/kubernetes/base/keycloak.yaml`
- Create: `infrastructure/kubernetes/base/postgresql.yaml`
- Create: `infrastructure/kubernetes/base/redis.yaml`
- Create: `infrastructure/kubernetes/base/bff.yaml`
- Create: `infrastructure/kubernetes/base/kustomization.yaml`

- [ ] **Step 1: Create namespace**

```yaml
# infrastructure/kubernetes/base/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: garudapass
```

- [ ] **Step 2: Create PostgreSQL deployment**

```yaml
# infrastructure/kubernetes/base/postgresql.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgresql
  namespace: garudapass
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgresql
  template:
    metadata:
      labels:
        app: postgresql
    spec:
      containers:
        - name: postgresql
          image: postgres:16-alpine
          ports:
            - containerPort: 5432
          env:
            - name: POSTGRES_USER
              valueFrom:
                secretKeyRef:
                  name: db-credentials
                  key: username
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: db-credentials
                  key: password
            - name: POSTGRES_DB
              value: garudapass
          volumeMounts:
            - name: pgdata
              mountPath: /var/lib/postgresql/data
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "1Gi"
              cpu: "1000m"
      volumes:
        - name: pgdata
          persistentVolumeClaim:
            claimName: postgresql-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: postgresql
  namespace: garudapass
spec:
  selector:
    app: postgresql
  ports:
    - port: 5432
      targetPort: 5432
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgresql-pvc
  namespace: garudapass
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

- [ ] **Step 3: Create Redis deployment**

```yaml
# infrastructure/kubernetes/base/redis.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
  namespace: garudapass
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
        - name: redis
          image: redis:7-alpine
          ports:
            - containerPort: 6379
          resources:
            requests:
              memory: "128Mi"
              cpu: "100m"
            limits:
              memory: "512Mi"
              cpu: "500m"
---
apiVersion: v1
kind: Service
metadata:
  name: redis
  namespace: garudapass
spec:
  selector:
    app: redis
  ports:
    - port: 6379
      targetPort: 6379
```

- [ ] **Step 4: Create Keycloak deployment**

```yaml
# infrastructure/kubernetes/base/keycloak.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: keycloak
  namespace: garudapass
spec:
  replicas: 1
  selector:
    matchLabels:
      app: keycloak
  template:
    metadata:
      labels:
        app: keycloak
    spec:
      containers:
        - name: keycloak
          image: quay.io/keycloak/keycloak:26.0
          args: ["start", "--import-realm"]
          ports:
            - containerPort: 8080
          env:
            - name: KC_DB
              value: postgres
            - name: KC_DB_URL
              value: jdbc:postgresql://postgresql:5432/garudapass
            - name: KC_DB_USERNAME
              valueFrom:
                secretKeyRef:
                  name: db-credentials
                  key: username
            - name: KC_DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: db-credentials
                  key: password
            - name: KC_HOSTNAME
              value: auth.garudapass.id
            - name: KC_HTTP_ENABLED
              value: "true"
            - name: KC_HEALTH_ENABLED
              value: "true"
            - name: KC_PROXY_HEADERS
              value: xforwarded
          resources:
            requests:
              memory: "512Mi"
              cpu: "500m"
            limits:
              memory: "2Gi"
              cpu: "2000m"
---
apiVersion: v1
kind: Service
metadata:
  name: keycloak
  namespace: garudapass
spec:
  selector:
    app: keycloak
  ports:
    - port: 8080
      targetPort: 8080
```

- [ ] **Step 5: Create BFF deployment**

```yaml
# infrastructure/kubernetes/base/bff.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bff
  namespace: garudapass
spec:
  replicas: 2
  selector:
    matchLabels:
      app: bff
  template:
    metadata:
      labels:
        app: bff
    spec:
      containers:
        - name: bff
          image: garudapass/bff:latest
          ports:
            - containerPort: 4000
          env:
            - name: BFF_PORT
              value: "4000"
            - name: KEYCLOAK_URL
              value: http://keycloak:8080
            - name: KEYCLOAK_REALM
              value: garudapass
            - name: KEYCLOAK_CLIENT_ID
              value: bff-client
            - name: REDIS_URL
              value: redis://redis:6379
            - name: BFF_SESSION_SECRET
              valueFrom:
                secretKeyRef:
                  name: bff-secrets
                  key: session-secret
            - name: BFF_FRONTEND_URL
              value: https://garudapass.id
            - name: BFF_REDIRECT_URI
              value: https://garudapass.id/auth/callback
          resources:
            requests:
              memory: "64Mi"
              cpu: "100m"
            limits:
              memory: "256Mi"
              cpu: "500m"
          livenessProbe:
            httpGet:
              path: /health
              port: 4000
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 4000
            initialDelaySeconds: 3
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: bff
  namespace: garudapass
spec:
  selector:
    app: bff
  ports:
    - port: 4000
      targetPort: 4000
```

- [ ] **Step 6: Create kustomization**

```yaml
# infrastructure/kubernetes/base/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: garudapass

resources:
  - namespace.yaml
  - postgresql.yaml
  - redis.yaml
  - keycloak.yaml
  - bff.yaml
```

- [ ] **Step 7: Validate manifests**

Run: `kubectl kustomize infrastructure/kubernetes/base/ > /dev/null && echo "Valid"`
Expected: `Valid` (no errors). If `kubectl` is not available, use `kustomize build`.

- [ ] **Step 8: Commit**

```bash
git add infrastructure/kubernetes/
git commit -m "feat: add Kubernetes base manifests for PostgreSQL, Redis, Keycloak, and BFF"
```

---

## Task 13: End-to-End Smoke Test

**Files:** None new — this verifies the full stack works together.

- [ ] **Step 1: Start all services**

Run: `docker compose up -d && sleep 20`
Expected: All containers healthy.

- [ ] **Step 2: Verify Keycloak realm**

Run: `curl -s http://localhost:8080/realms/garudapass/.well-known/openid-configuration | python3 -c "import sys,json; d=json.load(sys.stdin); print('PAR:', 'pushed_authorization_request_endpoint' in d); print('Issuer:', d.get('issuer',''))"`
Expected:
```
PAR: True
Issuer: http://localhost:8080/realms/garudapass
```

- [ ] **Step 3: Start BFF**

Run: `cd apps/bff && source ../../.env && go run . &`
Expected: `BFF listening on :4000`

- [ ] **Step 4: Verify BFF health**

Run: `curl -s http://localhost:4000/health`
Expected: `{"status":"ok"}`

- [ ] **Step 5: Verify session endpoint (unauthenticated)**

Run: `curl -s http://localhost:4000/auth/session | python3 -m json.tool`
Expected: `{"authenticated": false, "user": null}`

- [ ] **Step 6: Verify login redirect**

Run: `curl -s -o /dev/null -w "%{http_code} %{redirect_url}" http://localhost:4000/auth/login`
Expected: `302` with redirect URL containing `realms/garudapass/protocol/openid-connect/auth` and `code_challenge`.

- [ ] **Step 7: Run all Go tests**

Run: `cd apps/bff && go test ./... -v -count=1`
Expected: All tests PASS.

- [ ] **Step 8: Final commit**

```bash
git add -A
git commit -m "feat: complete Plan 1 foundation - monorepo, Keycloak FAPI 2.0, Go BFF, Next.js portals, K8s manifests"
```

---

## Summary

| Task | Component | Tests | Status |
|------|-----------|-------|--------|
| 1 | Monorepo scaffold | — | ☐ |
| 2 | Docker Compose dev env | — | ☐ |
| 3 | Keycloak FAPI 2.0 realm | Manual verify | ☐ |
| 4 | Kong API gateway config | Manual verify | ☐ |
| 5 | BFF config | 2 unit tests | ☐ |
| 6 | BFF session store | 1 unit test | ☐ |
| 7 | BFF auth handlers | 4 unit tests | ☐ |
| 8 | BFF CSRF middleware | 3 unit tests | ☐ |
| 9 | BFF main server | Compile check | ☐ |
| 10 | Citizen portal | Build check | ☐ |
| 11 | Admin portal | Build check | ☐ |
| 12 | Kubernetes manifests | Kustomize validate | ☐ |
| 13 | E2E smoke test | Integration | ☐ |

**Next:** Plan 2 (Identity Core) builds on this foundation with Dukcapil integration, GarudaInfo consent API, Keycloak custom SPI for NIK-based auth, and citizen portal features.
