# Phase 5: Developer Portal & API Gateway — Design Specification

**Version:** 1.0  
**Date:** 2026-04-06  
**Status:** Draft  
**Depends on:** Phase 1-4 (Foundation, Identity, Corporate, Signing)

---

## 1. Overview

Phase 5 adds the developer-facing layer of GarudaPass: a developer portal service for app registration, API key management, and usage tracking, plus a webhook system for async notifications. This is the critical enabler that transforms GarudaPass from an internal platform into a developer product.

### 1.1 Scope

**In scope:**
- GarudaPortal service: developer registration, app management, API key lifecycle, rate limit tiers
- API key authentication middleware for Kong gateway
- Sandbox environment with mock data (dev keys hit simulators automatically)
- Usage metering: per-app API call counting with daily/monthly aggregation
- Webhook system: event subscriptions, delivery with retry, signature verification
- App-level OAuth2 client registration (dynamic client registration)
- Developer dashboard APIs (usage stats, key management, webhook logs)

**Out of scope (deferred):**
- Billing/payment integration → Phase 7
- SDK generation (Go, Node, Python) → Phase 7
- Interactive API documentation (Swagger UI) → Phase 7
- Custom domain/branding for OAuth screens → future
- Team management (multi-developer per app) → future

### 1.2 Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Portal service | Go | Consistent with all GarudaPass services |
| API key format | `gp_live_` / `gp_test_` prefix + 48 random bytes (base62) | Prefix enables key type detection; base62 is URL-safe |
| Key storage | SHA-256 hash only — plaintext shown once at creation | Same pattern as GitHub/Stripe; compromised DB doesn't leak keys |
| Rate limit tiers | Free (100/day), Starter (10K/day), Growth (100K/day), Enterprise (custom) | Progressive pricing; generous free tier for adoption |
| Webhook signatures | HMAC-SHA256 with per-app secret | Industry standard (Stripe, GitHub); easy to verify |
| Webhook retry | Exponential backoff: 1m, 5m, 30m, 2h, 24h (5 attempts) | Standard retry schedule; avoids thundering herd |
| Usage storage | In-memory counters + daily flush to PostgreSQL (MVP) | ScyllaDB for high-volume metering deferred to Phase 2 |

---

## 2. Architecture

### 2.1 Service Topology

```
Developer → Developer Portal (Go) → PostgreSQL (apps, keys, webhooks, usage)
                                   → Redis (rate limit counters, key cache)
                                   → Kafka (webhook events, usage events)

API Consumer → Kong (API Gateway) → Kong plugin validates API key via Portal
                                   → Routes to internal services
                                   → Portal records usage
```

### 2.2 New Services

```
services/
└── garudaportal/    # Go — Developer portal service
```

### 2.3 Service Responsibilities

**GarudaPortal Service** (`services/garudaportal/`)
- Developer (app) registration: create app, get credentials
- API key lifecycle: create, list, revoke, rotate
- OAuth2 dynamic client registration for Keycloak
- Rate limit tier management
- Usage metering: increment counters, aggregate daily/monthly
- Webhook subscription management: subscribe, list, delete
- Webhook delivery: consume Kafka events, deliver with retry, log outcomes
- Key validation endpoint for Kong plugin

---

## 3. Data Model

### 3.1 Developer Apps Table

```sql
-- infrastructure/db/migrations/010_create_developer_apps.sql
CREATE TABLE IF NOT EXISTS developer_apps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    environment VARCHAR(10) NOT NULL DEFAULT 'sandbox', -- sandbox, production
    tier VARCHAR(20) NOT NULL DEFAULT 'free',            -- free, starter, growth, enterprise
    daily_limit INT NOT NULL DEFAULT 100,
    callback_urls TEXT[] NOT NULL DEFAULT '{}',
    oauth_client_id VARCHAR(100),                        -- Keycloak client ID
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',        -- ACTIVE, SUSPENDED, DELETED
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dev_apps_owner ON developer_apps(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_dev_apps_status ON developer_apps(status);
CREATE INDEX IF NOT EXISTS idx_dev_apps_oauth_client ON developer_apps(oauth_client_id);
```

### 3.2 API Keys Table

```sql
-- infrastructure/db/migrations/011_create_api_keys.sql
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES developer_apps(id),
    key_hash VARCHAR(64) NOT NULL,                       -- SHA-256 of plaintext key
    key_prefix VARCHAR(16) NOT NULL,                     -- gp_live_ or gp_test_ + first 8 chars
    name VARCHAR(100) NOT NULL DEFAULT 'Default',
    environment VARCHAR(10) NOT NULL,                    -- sandbox, production
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',        -- ACTIVE, REVOKED
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,                              -- optional expiry
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_keys_app ON api_keys(app_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix);
CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys(status);
```

### 3.3 Webhook Subscriptions Table

```sql
-- infrastructure/db/migrations/012_create_webhook_subscriptions.sql
CREATE TABLE IF NOT EXISTS webhook_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES developer_apps(id),
    url VARCHAR(2048) NOT NULL,
    events TEXT[] NOT NULL,                               -- e.g. {identity.verified, document.signed}
    secret VARCHAR(64) NOT NULL,                          -- HMAC signing secret (hex)
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',         -- ACTIVE, DISABLED
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhooks_app ON webhook_subscriptions(app_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_status ON webhook_subscriptions(status);
```

### 3.4 Webhook Deliveries Table

```sql
-- infrastructure/db/migrations/013_create_webhook_deliveries.sql
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID NOT NULL REFERENCES webhook_subscriptions(id),
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',        -- PENDING, DELIVERED, FAILED
    attempts INT NOT NULL DEFAULT 0,
    last_response_code INT,
    last_response_body TEXT,
    next_retry_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_deliveries_sub ON webhook_deliveries(subscription_id);
CREATE INDEX IF NOT EXISTS idx_deliveries_status ON webhook_deliveries(status);
CREATE INDEX IF NOT EXISTS idx_deliveries_retry ON webhook_deliveries(next_retry_at) WHERE status = 'PENDING';
```

### 3.5 API Usage Table

```sql
-- infrastructure/db/migrations/014_create_api_usage.sql
CREATE TABLE IF NOT EXISTS api_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES developer_apps(id),
    date DATE NOT NULL,
    endpoint VARCHAR(200) NOT NULL,
    call_count BIGINT NOT NULL DEFAULT 0,
    error_count BIGINT NOT NULL DEFAULT 0,
    UNIQUE(app_id, date, endpoint)
);

CREATE INDEX IF NOT EXISTS idx_usage_app_date ON api_usage(app_id, date);
```

---

## 4. API Design

### 4.1 App Management

```
POST   /api/v1/portal/apps
  Headers: X-User-ID: <user_id>
  Body: { name, description, callback_urls: ["https://..."] }
  Response 201: { id, name, environment: "sandbox", tier: "free", daily_limit: 100, status }

GET    /api/v1/portal/apps
  Headers: X-User-ID: <user_id>
  Response: { apps: [{ id, name, environment, tier, status, created_at }] }

GET    /api/v1/portal/apps/:id
  Headers: X-User-ID: <user_id>
  Response: { id, name, description, environment, tier, daily_limit, callback_urls, status }

PATCH  /api/v1/portal/apps/:id
  Headers: X-User-ID: <user_id>
  Body: { name?, description?, callback_urls? }
  Response: { ...updated app }
```

### 4.2 API Key Management

```
POST   /api/v1/portal/apps/:app_id/keys
  Headers: X-User-ID: <user_id>
  Body: { name: "Production Key", expires_in_days?: 365 }
  Response 201: {
    id, key_prefix, name, environment, status, created_at,
    plaintext_key: "gp_test_abc123..." // SHOWN ONCE ONLY
  }

GET    /api/v1/portal/apps/:app_id/keys
  Headers: X-User-ID: <user_id>
  Response: { keys: [{ id, key_prefix, name, status, last_used_at, expires_at }] }

DELETE /api/v1/portal/apps/:app_id/keys/:key_id
  Headers: X-User-ID: <user_id>
  Response 200: { id, status: "REVOKED", revoked_at }
```

### 4.3 Key Validation (Internal — Kong → Portal)

```
POST   /internal/keys/validate
  Body: { api_key: "gp_test_abc123..." }
  Response 200: { valid: true, app_id, environment, tier, daily_limit }
  Response 401: { valid: false, error: "invalid_key" }
  Response 429: { valid: false, error: "rate_limit_exceeded" }
```

### 4.4 Webhook Management

```
POST   /api/v1/portal/apps/:app_id/webhooks
  Headers: X-User-ID: <user_id>
  Body: { url: "https://...", events: ["identity.verified", "document.signed"] }
  Response 201: { id, url, events, status, secret: "whsec_..." }

GET    /api/v1/portal/apps/:app_id/webhooks
  Headers: X-User-ID: <user_id>
  Response: { webhooks: [{ id, url, events, status }] }

DELETE /api/v1/portal/apps/:app_id/webhooks/:webhook_id
  Headers: X-User-ID: <user_id>
  Response 200: { id, status: "DISABLED" }
```

### 4.5 Usage & Stats

```
GET    /api/v1/portal/apps/:app_id/usage
  Headers: X-User-ID: <user_id>
  Query: ?from=2026-04-01&to=2026-04-06
  Response: {
    total_calls: 1234,
    total_errors: 12,
    daily: [{ date, calls, errors }],
    by_endpoint: [{ endpoint, calls, errors }]
  }
```

---

## 5. API Key Design

### 5.1 Key Format

```
Prefix: gp_test_  (sandbox) or gp_live_  (production)
Random: 48 bytes encoded as base62 (64 chars)
Full key: gp_test_aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789aBcDeFgHiJkLmNoP (72 chars total)
```

### 5.2 Key Storage

```
1. Generate 48 random bytes (crypto/rand)
2. Encode as base62 string
3. Prepend prefix: gp_test_ or gp_live_
4. Compute SHA-256(full_key)
5. Store in DB: key_hash = SHA-256, key_prefix = first 16 chars of full key
6. Return full key to developer ONCE — never stored or logged in plaintext
```

### 5.3 Key Validation

```
1. Receive API key in X-API-Key header or Authorization: Bearer
2. Compute SHA-256(key)
3. Lookup key_hash in DB
4. Verify: status == ACTIVE, not expired, app status == ACTIVE
5. Check daily usage counter against daily_limit
6. Return app context (app_id, environment, tier)
```

---

## 6. Webhook Delivery

### 6.1 Signature Format

```
X-GarudaPass-Signature: t=1712400000,v1=5257a869e7ecebeda32affa62cdca3fa51cad7e77a0e56ff536d0ce8e108d8bd
```

Computed as: `HMAC-SHA256(timestamp + "." + payload_json, webhook_secret)`

### 6.2 Retry Schedule

| Attempt | Delay | Cumulative |
|---------|-------|-----------|
| 1 | Immediate | 0 |
| 2 | 1 minute | 1m |
| 3 | 5 minutes | 6m |
| 4 | 30 minutes | 36m |
| 5 | 2 hours | 2h 36m |
| 6 (final) | 24 hours | 26h 36m |

### 6.3 Event Types

```
identity.verified        — user completed identity verification
identity.consent.granted — user granted data consent
identity.consent.revoked — user revoked data consent
corp.entity.verified     — corporate entity verified
corp.role.assigned       — corporate role assigned
corp.role.revoked        — corporate role revoked
sign.certificate.issued  — signing certificate issued
sign.document.signed     — document signed
sign.document.failed     — document signing failed
```

---

## 7. Security

### 7.1 Key Security
- API keys hashed (SHA-256) at rest — plaintext never stored
- Key prefix stored separately for identification without revealing full key
- Rate limiting per key (daily) and per IP (per-minute)
- Immediate revocation: key validation checks status on every request
- Key rotation: create new key, migrate traffic, revoke old key

### 7.2 Webhook Security
- Per-subscription HMAC secret (32 random bytes, hex-encoded)
- Timestamp in signature prevents replay attacks (5-minute tolerance)
- HTTPS required for webhook URLs (enforced at subscription creation)
- Response body truncated in delivery log (max 1KB) to limit data leakage

### 7.3 App Isolation
- Apps see only their own keys, webhooks, and usage data
- Owner validation on every request (X-User-ID must match app.owner_user_id)
- Sandbox keys can only access simulator endpoints
- Production keys require app status = ACTIVE and tier ≥ starter

---

## 8. Testing Strategy

| Layer | Approach |
|-------|----------|
| Key generation | Verify format, uniqueness, hash storage, prefix extraction |
| Key validation | CRUD, expiry, revocation, rate limit checking |
| App store | CRUD, ownership enforcement, status transitions |
| Webhook store | CRUD, event filtering, subscription management |
| Webhook delivery | Signature computation, retry scheduling, HTTP delivery mock |
| Usage metering | Counter increment, daily aggregation, limit enforcement |
| Handler tests | httptest for all endpoints, auth checks, error cases |
| Coverage target | >= 80% per package |

---

## 9. Configuration (new env vars)

```bash
# GarudaPortal Service
GARUDAPORTAL_PORT=4009
GARUDAPORTAL_DB_URL=postgres://garudapass:garudapass@localhost:5432/garudapass
IDENTITY_SERVICE_URL=http://localhost:4001
KEYCLOAK_ADMIN_URL=http://localhost:8080
KEYCLOAK_ADMIN_USER=admin
KEYCLOAK_ADMIN_PASSWORD=admin
KAFKA_BROKERS=localhost:19092
REDIS_URL=redis://:garudapass-redis-dev@localhost:6379
```

---

## 10. File Structure

```
services/garudaportal/
├── go.mod
├── main.go
├── Dockerfile
├── config/
│   ├── config.go
│   └── config_test.go
├── apikey/
│   ├── generator.go          # Key generation (crypto/rand + base62 + prefix)
│   ├── generator_test.go
│   ├── validator.go           # Key validation (hash lookup + status + rate limit)
│   └── validator_test.go
├── store/
│   ├── app.go                 # AppStore interface + InMemory
│   ├── app_test.go
│   ├── key.go                 # KeyStore interface + InMemory
│   ├── key_test.go
│   ├── webhook.go             # WebhookStore interface + InMemory
│   ├── webhook_test.go
│   ├── delivery.go            # DeliveryStore interface + InMemory
│   ├── delivery_test.go
│   ├── usage.go               # UsageStore interface + InMemory
│   └── usage_test.go
├── webhook/
│   ├── signer.go              # HMAC-SHA256 signature computation
│   ├── signer_test.go
│   ├── dispatcher.go          # HTTP delivery with retry scheduling
│   └── dispatcher_test.go
├── handler/
│   ├── app.go                 # App CRUD handlers
│   ├── app_test.go
│   ├── key.go                 # Key management handlers
│   ├── key_test.go
│   ├── webhook.go             # Webhook subscription handlers
│   ├── webhook_test.go
│   ├── usage.go               # Usage stats handlers
│   ├── usage_test.go
│   ├── validate.go            # Internal key validation handler
│   └── validate_test.go

infrastructure/db/migrations/
├── 010_create_developer_apps.sql
├── 011_create_api_keys.sql
├── 012_create_webhook_subscriptions.sql
├── 013_create_webhook_deliveries.sql
└── 014_create_api_usage.sql
```

---

## 11. Docker Compose Additions

```yaml
  garudaportal:
    build: ./services/garudaportal
    restart: unless-stopped
    ports:
      - "4009:4009"
    env_file: .env
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    deploy:
      resources:
        limits:
          memory: 256M
          cpus: "0.5"
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    networks:
      - gpass-network
```

---

## 12. Go Workspace Update

```
go 1.25.0

use (
    ...existing...
    ./services/garudaportal
)
```
