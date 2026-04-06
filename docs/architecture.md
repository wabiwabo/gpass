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
| BFF | 124 | httptest, miniredis, proxy, CSRF, session security, health aggregation |
| Identity | 101 | Dukcapil mock, OTP, deletion (UU PDP), export, NIK validation edge cases |
| GarudaInfo | 53 | Field-level consent, multi-user isolation, grant/revoke lifecycle |
| GarudaCorp | 93 | Role hierarchy, UBO (PP 13/2018), threshold boundary tests |
| GarudaSign | 95 | Mock PAdES-B-LTA, certificate lifecycle, revocation edge cases |
| GarudaPortal | 181 | API keys, webhooks, worker, rotation, tier validation |
| GarudaAudit | 68 | Immutable append-only, PP 71/2019 compliance, stats |
| GarudaNotify | 70 | Email + SMS channels, templates, batch, validation |
| golib | 1366 | 80 packages, race-tested, enterprise patterns |
| Simulators | 76 | Synthetic data, cross-referencing NIKs, edge cases |
| Integration | 16 | E2E flows: signing, portal, audit, identity, consent, corporate |
| **Total** | **2,243** | |

## golib Shared Library (80 packages)

### Security (10 packages)
| Package | Purpose |
|---------|---------|
| `crypto` | HMAC-SHA256, random bytes, multi-version KeyRing |
| `digest` | SHA-256/384/512, CRC32, file hashing, multi-hash verification |
| `fingerprint` | Request fingerprinting for bot/abuse detection |
| `jwt` | ECDSA P-256 JWT signing/verification, JWKS endpoint |
| `kms` | Envelope encryption (DEK/KEK) with local provider |
| `mask` | Indonesian PII masking (NIK, email, phone, NPWP) |
| `mtls` | Mutual TLS client with test certificate generation |
| `permission` | Scope-based access control with role hierarchy |
| `pii` | AES-256-GCM field encryption, masking, hash lookup |
| `sanitize` | XSS, SQL injection, path traversal protection |

### Resilience (8 packages)
| Package | Purpose |
|---------|---------|
| `adaptive` | Error-rate-aware throttle with auto backoff/recovery |
| `budget` | Time and call-count budget enforcement |
| `bulkhead` | Semaphore-based concurrency isolation |
| `circuitbreaker` | Fault isolation with advanced half-open probing |
| `distlock` | Distributed mutex with fencing tokens |
| `ratelimit` | Token bucket + sliding window rate limiter |
| `resilience` | Fallback[T], Retry[T] with exponential backoff |
| `singleflight` | Request coalescing for duplicate suppression |

### Observability (8 packages)
| Package | Purpose |
|---------|---------|
| `accesslog` | Request latency percentile tracking (p50/p95/p99) |
| `depcheck` | Concurrent dependency health checking |
| `healthgraph` | Service dependency DAG with cascade analysis |
| `logging` | Structured logger with PII redaction and sampling |
| `metrics` | Prometheus-compatible HTTP metrics middleware |
| `probe` | K8s liveness/readiness/startup probe manager |
| `tags` | Thread-safe observability tag propagation |
| `tracing` | W3C Trace Context with span tracking |

### API Standards (9 packages)
| Package | Purpose |
|---------|---------|
| `apiresponse` | RFC 7807 Problem Details responses |
| `cursor` | Opaque cursor-based pagination with generics |
| `httputil` | JSON, pagination, filtering, batch API, versioned router |
| `negotiate` | HTTP content negotiation (Accept, Encoding, Language) |
| `pagination` | Offset-based pagination with generics Apply[T] |
| `rateheader` | IETF RateLimit-* header utilities |
| `reqvalidator` | Structured request validation with field errors |
| `respwriter` | Response writer wrappers (Capture, Buffer, Pool) |
| `webhook` | Ed25519 webhook signing (v2) |

### Data Management (8 packages)
| Package | Purpose |
|---------|---------|
| `audittrail` | Fluent audit entry builder with 13 action types |
| `cqrs` | Command/query separation with bus dispatchers |
| `eventsource` | Event sourcing with aggregate, snapshot, repository |
| `idempotent` | Request deduplication with idempotency keys |
| `lineage` | Data flow tracking for UU PDP compliance |
| `outbox` | Transactional outbox pattern with poller |
| `retention` | PP 71/2019 and UU PDP retention policy enforcer |
| `seeddata` | JSON seed data loader with dependency ordering |

### Infrastructure (17 packages)
| Package | Purpose |
|---------|---------|
| `audit` | Audit context enrichment, IP extraction |
| `batch` | Generic concurrent batch processing |
| `bootstrap` | Standardized service startup with middleware chain |
| `cache` | TTL cache with GetOrSet, stats |
| `config` | Runtime config values with atomic swap |
| `configloader` | Struct-tag config loading from env vars |
| `database` | PostgreSQL pool, transactions, migrations, query builder |
| `environ` | Typed environment variable helpers |
| `errors` | Structured errors with stack traces, 30+ codes |
| `events` | Event bus abstraction (LogPublisher, MemoryBus) |
| `health` | Concurrent health checks, IETF format |
| `httpclient` | HTTP client with circuit breaker, request signing |
| `redis` | Redis client with health checking |
| `requestid` | Request ID generation and middleware |
| `server` | Graceful shutdown server with connection draining |
| `shutdown` | Shutdown coordinator with priority-ordered hooks |
| `token` | Cryptographic token generation (hex, base62, UUID, OTP) |

### Domain (10 packages)
| Package | Purpose |
|---------|---------|
| `chaos` | Fault injection with configurable error rates |
| `contract` | Consumer-driven contract testing framework |
| `dlq` | Dead letter queue with HTTP management |
| `featureflags` | Runtime flags with percentage rollout |
| `middleware` | 25+ middlewares: Recovery, RBAC, CORS, CSRF, SignVerify, Enrich, ErrorChain, etc. |
| `oauth2` | Token introspection, JWKS, OIDC Discovery, Token Exchange |
| `policy` | ABAC policy engine with wildcard matching |
| `propagation` | Context propagation (8 headers) for inter-service calls |
| `quota` | Monthly API quota management per tier |
| `registry` | Service discovery with heartbeat |
| `scheduler` | Periodic background job execution |
| `sdkgen` | Go SDK code generator from API definitions |
| `security` | Security event logger for SOC/SIEM (16 event types) |
| `tenant` | Multi-tenant isolation middleware |
| `timeutil` | Indonesian timezone support (WIB/WITA/WIT) |
| `validate` | Input validation (NIK, email, UUID, password strength) |
| `worker` | Generic worker pool for background tasks |
| `workqueue` | Priority work queue with backpressure |
