# GarudaPass Architecture Overview

## Service Map

```
                         ┌─────────────┐
                         │   Kong API  │
                         │   Gateway   │
                         │  (port 8000)│
                         └──────┬──────┘
                                │
                    ┌───────────┴───────────┐
                    │                       │
              ┌─────▼─────┐          ┌─────▼─────┐
              │  Web App  │          │ Admin App │
              │ Next.js   │          │ Next.js   │
              │ (port 3000)│         │(port 3001)│
              └─────┬─────┘          └─────┬─────┘
                    │                       │
                    └───────────┬───────────┘
                                │
                         ┌──────▼──────┐
                         │    BFF      │
                         │  (port 4000)│
                         │ Auth/Session│
                         └──────┬──────┘
                                │
          ┌─────────┬───────────┼───────────┬──────────┐
          │         │           │           │          │
    ┌─────▼───┐ ┌───▼───┐ ┌────▼────┐ ┌────▼────┐ ┌──▼──────┐
    │Identity │ │Garuda │ │Garuda  │ │Garuda  │ │Garuda  │
    │ (4001)  │ │Info   │ │Corp    │ │Sign    │ │Portal  │
    │         │ │(4003) │ │(4006)  │ │(4007)  │ │(4009)  │
    └────┬────┘ └───┬───┘ └───┬────┘ └───┬────┘ └───┬────┘
         │          │     ┌───┴────┐     │          │
    ┌────▼────┐     │ ┌───▼──┐┌───▼──┐┌──▼────┐    │
    │Dukcapil │     │ │AHU   ││OSS   ││Sign   │    │
    │ Sim     │     │ │Sim   ││Sim   ││Sim    │    │
    │ (4002)  │     │ │(4004)││(4005)││(4008) │    │
    └─────────┘     │ └──────┘└──────┘└───────┘    │
                    │                               │
              ┌─────▼─────────────────────────▼─────┐
              │           PostgreSQL                 │
              │            (5432)                    │
              └──────────────────────────────────────┘
              ┌──────────────────────────────────────┐
              │             Redis (6379)              │
              └──────────────────────────────────────┘
              ┌──────────────────────────────────────┐
              │          Redpanda/Kafka (19092)       │
              └──────────────────────────────────────┘
```

## Security Architecture

```
External Request
    │
    ▼
┌──────────┐  Rate Limit   ┌──────────┐  CSRF Check   ┌──────────┐
│   Kong   │──────────────▶│   BFF    │──────────────▶│ Session  │
│ Gateway  │  Bot Detect   │          │  Constant-time │ Store    │
│          │  Size Limit   │          │  Compare       │ (Redis)  │
│          │  Security Hdr │          │                │ AES-256  │
└──────────┘               └──────────┘                └──────────┘
                                │
                          Session Cookie
                          (HttpOnly, Secure,
                           SameSite=Strict)
                                │
                                ▼
                     Internal Services
                     (X-User-ID header)
```

## Data Protection

| Data | Protection Method |
|------|------------------|
| NIK (National ID) | HMAC-SHA256 tokenization — never stored as plaintext |
| PII fields | AES-256-GCM envelope encryption (DEK/KEK) |
| Session data | AES-256-GCM encrypted in Redis |
| API keys | SHA-256 hash only — plaintext shown once |
| OTP codes | bcrypt hash in Redis with TTL |
| Signing keys | Never leave CA boundary (EJBCA/HSM) |
| Webhook secrets | HMAC-SHA256 per subscription |

## Compliance

| Regulation | Implementation |
|-----------|---------------|
| UU PDP No. 27/2022 | Field-level consent, data deletion on revocation |
| PP 71/2019 | 5-year audit trail retention via Kafka |
| PP 13/2018 | Beneficial ownership (UBO) analysis, 25% threshold |
| ETSI EN 319 142 | PAdES-B-LTA digital signatures |
| FAPI 2.0 | PAR, DPoP, PKCE S256, private_key_jwt |

## Port Allocation

| Port | Service |
|------|---------|
| 3000 | Citizen Portal (Next.js) |
| 3001 | Admin Dashboard (Next.js) |
| 3002 | Grafana |
| 4000 | BFF |
| 4001 | Identity Service |
| 4002 | Dukcapil Simulator |
| 4003 | GarudaInfo |
| 4004 | AHU Simulator |
| 4005 | OSS Simulator |
| 4006 | GarudaCorp |
| 4007 | GarudaSign |
| 4008 | Signing Simulator |
| 4009 | GarudaPortal |
| 4010 | GarudaAudit |
| 4011 | GarudaNotify |
| 5432 | PostgreSQL |
| 6379 | Redis |
| 8000 | Kong Proxy |
| 8001 | Kong Admin |
| 8080 | Keycloak |
| 9090 | Prometheus |
| 16686 | Jaeger UI |
| 19092 | Redpanda/Kafka |

## Database Migrations

| # | Table | Service |
|---|-------|---------|
| 001 | users | Identity |
| 002 | consents | GarudaInfo |
| 003 | entities | GarudaCorp |
| 004 | entity_officers | GarudaCorp |
| 005 | entity_roles | GarudaCorp |
| 006 | entity_shareholders | GarudaCorp |
| 007 | signing_certificates | GarudaSign |
| 008 | signing_requests | GarudaSign |
| 009 | signed_documents | GarudaSign |
| 010 | developer_apps | GarudaPortal |
| 011 | api_keys | GarudaPortal |
| 012 | webhook_subscriptions | GarudaPortal |
| 013 | webhook_deliveries | GarudaPortal |
| 014 | api_usage | GarudaPortal |
| 015 | beneficial_owners | GarudaCorp (UBO) |
| 016 | audit_events | GarudaAudit |

## Test Coverage

| Module | Tests | Key Patterns |
|--------|-------|-------------|
| BFF | 74 | httptest, miniredis, proxy, health aggregation |
| Identity | 66 | Dukcapil mock, OTP, deletion (UU PDP), export, repositories |
| GarudaInfo | 18 | Field-level consent filtering |
| GarudaCorp | 72 | Role hierarchy, UBO (PP 13/2018) |
| GarudaSign | 48 | Mock PAdES-B-LTA, hash verification |
| GarudaPortal | 113 | API keys, webhooks, worker, repositories |
| GarudaAudit | 27 | Immutable append-only, stats |
| GarudaNotify | 16 | Email + SMS channels |
| golib | 290 | 20 packages, race-tested, enterprise middleware |
| Simulators | 44 | Synthetic data, cross-referencing NIKs |
| Integration | 4 | Contract validation |
| **Total** | **772** | |

## golib Shared Library (20 packages)

| Package | Purpose |
|---------|---------|
| `audit` | Audit context enrichment, IP extraction |
| `cache` | TTL cache with GetOrSet, stats |
| `circuitbreaker` | Fault isolation for external calls |
| `config` | Runtime config values with change listeners |
| `crypto` | HMAC-SHA256, random bytes, constant-time verify |
| `database` | PostgreSQL pool, transactions, migrations, query builder |
| `errors` | Structured errors with 30+ standard codes |
| `events` | Event bus (Kafka abstraction, MemoryBus) |
| `featureflags` | Runtime flags, percentage-based rollout |
| `health` | Concurrent health checks |
| `httpclient` | HTTP client with circuit breaker |
| `httputil` | JSON, pagination, filtering, request parsing |
| `middleware` | Recovery, RBAC, CORS, HSTS, Timeout, Idempotency, ServiceAuth, RateLimit, Correlation, APIVersion, SecureHeaders, RequestValidation |
| `pii` | AES-256-GCM field encryption, masking, hash lookup |
| `ratelimit` | Token bucket rate limiter |
| `redis` | Redis client with health checking |
| `resilience` | Fallback, Retry with exponential backoff |
| `sanitize` | XSS, SQL injection, path traversal protection |
| `server` | Graceful shutdown server |
| `validate` | Input validation (NIK, email, UUID, URL) |
