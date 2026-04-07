# GarudaPass Development Guide

## Project Structure

Turborepo monorepo with Go workspace for backend services.

```
apps/
  bff/              — Go Backend-for-Frontend (auth, sessions, API proxy, port 4000)
  web/              — Next.js citizen portal (port 3000)
  admin/            — Next.js admin dashboard (port 3001)

services/
  identity/         — Identity verification, NIK crypto, OTP (port 4001)
  dukcapil-sim/     — Dukcapil population data simulator (port 4002)
  garudainfo/       — Consent management, personal data API (port 4003)
  ahu-sim/          — AHU/SABH legal entity simulator (port 4004)
  oss-sim/          — OSS/BKPM business license simulator (port 4005)
  garudacorp/       — Corporate identity, roles, UBO analysis (port 4006)
  garudasign/       — Digital signing, PAdES-B-LTA (port 4007)
  signing-sim/      — Signing CA/PAdES simulator (port 4008)
  garudaportal/     — Developer portal, API keys, webhooks (port 4009)
  garudaaudit/      — Immutable audit trail (port 4010)
  garudanotify/     — Email/SMS notifications (port 4011)

packages/
  golib/            — Shared Go library (274 packages, stdlib only)
  ui/               — Shared React/shadcn components
  config/           — Shared TypeScript/Tailwind config

infrastructure/
  db/migrations/    — PostgreSQL migrations (001-016)
  db/seed/          — Development seed data
  keycloak/         — FAPI 2.0 realm configuration
  kong/             — API gateway with security headers
  kubernetes/       — K8s manifests (base + dev/staging/prod overlays)
  prometheus/       — Metrics scraping + SLO alerts
  grafana/          — Dashboard provisioning
  terraform/        — AWS IaC (VPC, EKS, RDS, ElastiCache, ECR)
  security/         — Security policy, CORS documentation

tests/
  integration/      — E2E flows (signing, portal, audit, identity, consent, corporate)
  load/             — k6 load tests (smoke, stress, soak)

docs/
  architecture.md   — Architecture overview + service map
  api/              — OpenAPI 3.1 specs per service
  operations/       — DR plan + runbook
  superpowers/      — Design specs + implementation plans
```

## Common Commands

```bash
make setup          # First-time setup
make up             # Start Docker Compose (24 services)
make test           # Run all Go tests (2,948+ tests)
make test-race      # Run with race detector
make test-count     # Count all tests across services
make cover          # Coverage report per service
make vet            # Go vet all services
make docker-build   # Build all Docker images
make migrate        # Run database migrations
make load-smoke     # k6 smoke test (all health endpoints)
make load-stress    # k6 stress test (200 VUs)
make load-soak      # k6 soak test (30 min)
make help           # Show all targets
```

## Development

```bash
# Run specific service tests
cd services/garudasign && go test ./... -v

# Run with race detector
cd packages/golib && go test ./... -race -count=1

# Build and verify
cd services/garudacorp && go build -o /dev/null .
```

### Test Coverage Requirements

- Minimum 80% overall coverage per service
- golib: 100% interface coverage, race-free

### Standard Service Contract

Every backend service in `services/` (and BFF) MUST expose these
unauthenticated endpoints. They are scraped by kubelet, Prometheus,
and CI smoke tests:

| Endpoint | Purpose | Body |
|---|---|---|
| `GET /health` | Liveness probe — process is up | `{"status":"ok",...}` |
| `GET /readyz` | Readiness probe — DB pingable | `{"status":"ok","db":"postgres",...}` |
| `GET /metrics` | Prometheus text format | counters + gauges + histogram |
| `GET /version` | Build identification | `{"service","commit","go_version",...}` |

Standard middleware chain (outermost first), provided by `packages/golib/httpx`:

```
SecurityHeaders → RequestID → AccessLog → Metrics.Instrument
  → Recover → MaxBodyBytes → Timeout(30s) → mux
```

Bringing up a new service: copy `services/garudaaudit/main.go` as a
template, swap the service name, replace handlers. The middleware,
endpoints, panic recovery, request IDs, structured logs, metrics,
and security headers are all inherited from `golib/httpx`.

### Security Architecture

- Sessions: AES-256-GCM encrypted in Redis
- CSRF: double-submit cookie with constant-time comparison
- Auth: FAPI 2.0 (PAR, DPoP, PKCE S256, private_key_jwt)
- NIK: HMAC-SHA256 tokenization (never stored plaintext)
- PII: AES-256-GCM field-level envelope encryption (DEK/KEK)
- API keys: SHA-256 hashed at rest, plaintext shown once
- Webhooks: HMAC-SHA256 signatures with timestamp tolerance
- Service-to-service: HMAC request signing
- Headers: CSP, HSTS preload, X-Frame-Options DENY, Permissions-Policy
- Cookies: HttpOnly, Secure (prod/staging), SameSite=Strict

### Compliance

- UU PDP No. 27/2022: field-level consent, deletion, export, privacy transparency
- PP 71/2019: 5-year immutable audit retention
- PP 13/2018: beneficial ownership (UBO) 25% threshold analysis
- ETSI EN 319 142: PAdES-B-LTA digital signatures
- RFC 5280: certificate revocation
- RFC 7662: token introspection

## Environment Variables

Copy `.env.example` to `.env`. See `.env.example` for all variables.

## Conventions

- Go: standard library HTTP (`net/http`), `log/slog` for structured JSON logging
- HTTP routing: Go 1.22+ method routing (`"GET /path"`, `"POST /path"`)
- Tests: table-driven, `httptest` for HTTP, `miniredis` for Redis, InMemory stores
- DI: interface-based dependency injection, constructor functions
- Errors: structured `AppError` with HTTP status and machine-readable codes
- Commits: conventional commits (`feat:`, `fix:`, `docs:`, `chore:`)
- No mocks for external services — use simulators, `httptest`, InMemory stores
- All SQL: parameterized queries ($1, $2) — never string interpolation
- All PII: encrypted at rest, tokenized for lookup
