# GarudaPass Development Guide

## Project Structure

Turborepo monorepo with Go workspace for backend services.

- `apps/bff/` — Go Backend-for-Frontend (auth, sessions, API proxy)
- `apps/citizen-portal/` — Next.js citizen-facing app (port 3000)
- `apps/admin-portal/` — Next.js admin app (port 3001)
- `packages/contracts/` — Shared TypeScript types
- `packages/ui/` — Shared React components
- `infrastructure/` — Keycloak, Kong, Kafka, Kubernetes configs

## Common Commands

```bash
make setup          # First-time setup
make up             # Start Docker Compose services
make test           # Run all tests
make cover          # Run tests with coverage report
make build          # Build BFF binary with version info
make docker-build   # Build BFF Docker image
make help           # Show all targets
```

## BFF Development

```bash
cd apps/bff
go test ./... -v          # Run tests
go test ./... -race       # Run with race detector
go vet ./...              # Static analysis
go build -o /dev/null .   # Verify compilation
```

### Test Coverage Requirements

- Minimum 80% overall coverage (enforced in CI)
- Config: 100%, Handler: 85%+, Middleware: 87%+, Session: 86%+

### Security Architecture

- Sessions encrypted with AES-256-GCM in Redis
- CSRF: double-submit cookie with constant-time comparison
- Auth: OAuth2 PKCE S256 + state validation
- Cookies: HttpOnly, Secure (prod/staging), SameSite=Strict
- HSTS enabled in production/staging

## Environment Variables

Copy `.env.example` to `.env`. Required for BFF:
- `KEYCLOAK_URL`, `KEYCLOAK_REALM`, `KEYCLOAK_CLIENT_ID`
- `BFF_SESSION_SECRET` (min 32 chars)
- `BFF_FRONTEND_URL`, `BFF_REDIRECT_URI`
- `REDIS_URL` (with auth in non-dev environments)

## Conventions

- Go: standard library HTTP, `log/slog` for structured logging
- Tests: table-driven where appropriate, `miniredis` for Redis tests
- Commits: conventional commits (`feat:`, `fix:`, `docs:`)
- No mocks for external services — use `miniredis`, `httptest`, etc.
