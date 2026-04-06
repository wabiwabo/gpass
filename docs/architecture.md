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
| golib | 1967 | 130 packages, race-tested, enterprise patterns |
| Simulators | 76 | Synthetic data, cross-referencing NIKs, edge cases |
| Integration | 16 | E2E flows: signing, portal, audit, identity, consent, corporate |
| **Total** | **2,903** | |

## golib Shared Library (130 packages)

### Security (13 packages)
| Package | Purpose |
|---------|---------|
| `crypto` | HMAC-SHA256, random bytes, multi-version KeyRing |
| `cryptoutil` | Constant-time comparison, HMAC, key derivation, secure random |
| `digest` | SHA-256/384/512, CRC32, file hashing, multi-hash verification |
| `fingerprint` | Request fingerprinting for bot/abuse detection |
| `ipallow` | IP allowlist/blocklist with CIDR ranges and HTTP middleware |
| `jwt` | ECDSA P-256 JWT signing/verification, JWKS endpoint |
| `kms` | Envelope encryption (DEK/KEK) with local provider |
| `mask` | Indonesian PII masking (NIK, email, phone, NPWP) |
| `mtls` | Mutual TLS client with test certificate generation |
| `permission` | Scope-based access control with role hierarchy |
| `pii` | AES-256-GCM field encryption, masking, hash lookup |
| `reqsign` | HMAC request signing for service-to-service authentication |
| `sanitize` | XSS, SQL injection, path traversal protection |

### Resilience (17 packages)
| Package | Purpose |
|---------|---------|
| `adaptive` | Error-rate-aware throttle with auto backoff/recovery |
| `backpressure` | System-level load shedding with priority admission control |
| `budget` | Time and call-count budget enforcement |
| `bulkhead` | Semaphore-based concurrency isolation |
| `cascade` | Cascading fallback for multi-source data (cache→db→API) |
| `circuitbreaker` | Fault isolation with advanced half-open probing |
| `circuithttp` | Circuit-breaker-aware HTTP transport with state callbacks |
| `circuitstate` | Circuit breaker state persistence for restart resilience |
| `connpool` | Generic connection pool with health check, lifetime, idle timeout |
| `drain` | Graceful connection draining for zero-downtime deploys |
| `distlock` | Distributed mutex with fencing tokens |
| `ratelimit` | Token bucket + sliding window rate limiter |
| `resilience` | Fallback[T], Retry[T] with exponential backoff |
| `semaphore` | Weighted counting semaphore with context cancellation |
| `singleflight` | Request coalescing for duplicate suppression |
| `throttle` | Per-key token bucket rate limiter with HTTP middleware |
| `webhookretry` | Exponential backoff retry for webhook delivery |

### Observability (16 packages)
| Package | Purpose |
|---------|---------|
| `accesslog` | Request latency percentile tracking (p50/p95/p99) |
| `audithttp` | Automatic HTTP audit event emission (PP 71/2019) |
| `correlation` | Structured correlation/causation ID propagation across services |
| `depcheck` | Concurrent dependency health checking |
| `healthagg` | Health aggregation across services with caching and HTTP handler |
| `healthgraph` | Service dependency DAG with cascade analysis |
| `logging` | Structured logger with PII redaction and sampling |
| `metrics` | Prometheus-compatible HTTP metrics middleware |
| `probe` | K8s liveness/readiness/startup probe manager |
| `ratewindow` | Fixed-window event counting for monitoring/analytics |
| `reqlog` | Structured HTTP request/response logging with PII filtering |
| `statuspage` | Service status page with components, incidents, maintenance |
| `tags` | Thread-safe observability tag propagation |
| `tracing` | W3C Trace Context with span tracking |
| `tracehttp` | Automatic trace header injection for downstream calls |
| `headerprop` | Standardized header propagation across service boundaries |

### API Standards (19 packages)
| Package | Purpose |
|---------|---------|
| `apiresponse` | RFC 7807 Problem Details responses |
| `apiver` | API versioning via header/path/query with deprecation |
| `compress` | HTTP response compression (gzip/deflate) with min size |
| `cursor` | Opaque cursor-based pagination with generics |
| `etag` | HTTP conditional requests with ETag/If-None-Match (RFC 7232) |
| `httputil` | JSON, pagination, filtering, batch API, versioned router |
| `jsonpatch` | RFC 6902 JSON Patch for partial resource updates |
| `maxbody` | Request body size enforcement with per-route limits |
| `multipart` | Safe multipart form handling with file upload validation |
| `negotiate` | HTTP content negotiation (Accept, Encoding, Language) |
| `paginatedb` | SQL pagination helpers for offset and keyset pagination |
| `pagination` | Offset-based pagination with generics Apply[T] |
| `rateheader` | IETF RateLimit-* header utilities |
| `reqvalidator` | Structured request validation (11 patterns: email, NIK, NPWP, etc.) |
| `structured` | RFC 7807 structured error reporting with field-level errors |
| `timeout` | Per-route HTTP timeout middleware with 504 responses |
| `respwriter` | Response writer wrappers (Capture, Buffer, Pool) |
| `webhook` | Ed25519 webhook signing (v2) |

### Data Management (10 packages)
| Package | Purpose |
|---------|---------|
| `audittrail` | Fluent audit entry builder with 13 action types |
| `cqrs` | Command/query separation with bus dispatchers |
| `dedup` | Exactly-once processing with content-hash deduplication |
| `envelope` | Standardized event envelope for Kafka with routing, retry, integrity |
| `eventsource` | Event sourcing with aggregate, snapshot, repository |
| `idempotent` | Request deduplication with idempotency keys |
| `lineage` | Data flow tracking for UU PDP compliance |
| `outbox` | Transactional outbox pattern with poller |
| `retention` | PP 71/2019 and UU PDP retention policy enforcer |
| `seeddata` | JSON seed data loader with dependency ordering |

### Infrastructure (26 packages)
| Package | Purpose |
|---------|---------|
| `audit` | Audit context enrichment, IP extraction |
| `batch` | Generic concurrent batch processing |
| `bloom` | Probabilistic set membership for dedup at scale |
| `bootstrap` | Standardized service startup with middleware chain |
| `cache` | TTL cache with GetOrSet, stats |
| `config` | Runtime config values with atomic swap |
| `configloader` | Struct-tag config loading from env vars |
| `contextutil` | Context helpers: Detach, Merge, deadline splitting |
| `database` | PostgreSQL pool, transactions, migrations, query builder |
| `environ` | Typed environment variable helpers |
| `errors` | Structured errors with stack traces, 30+ codes |
| `events` | Event bus abstraction (LogPublisher, MemoryBus) |
| `health` | Concurrent health checks, IETF format |
| `interval` | Periodic task execution with jitter and error stats |
| `httpclient` | HTTP client with circuit breaker, request signing |
| `iterset` | Generic set operations (union, intersection, difference) |
| `mwchain` | Composable middleware chain with named stages |
| `recovery` | Panic recovery with RFC 7807 error responses |
| `redis` | Redis client with health checking |
| `reqscope` | Request-scoped dependency injection |
| `requestid` | Request ID generation and middleware |
| `safemap` | Type-safe concurrent map with TTL and generics |
| `server` | Graceful shutdown server with connection draining |
| `shutdown` | Shutdown coordinator with priority-ordered hooks |
| `signctx` | Signal-aware context for graceful shutdown propagation |
| `sniff` | Content type sniffing prevention + security headers |
| `token` | Cryptographic token generation (hex, base62, UUID, OTP) |
| `typedctx` | Type-safe context values using generics |

### Domain (12 packages)
| Package | Purpose |
|---------|---------|
| `chaos` | Fault injection with configurable error rates |
| `contract` | Consumer-driven contract testing framework |
| `dlq` | Dead letter queue with HTTP management |
| `featureflags` | Runtime flags with percentage rollout |
| `middleware` | 25+ middlewares: Recovery, RBAC, CORS, CSRF, SignVerify, Enrich, ErrorChain, etc. |
| `normalize` | Indonesian name, phone, email, NIK, NPWP, address normalization |
| `oauth2` | Token introspection, JWKS, OIDC Discovery, Token Exchange |
| `policy` | ABAC policy engine with wildcard matching |
| `propagation` | Context propagation (8 headers) for inter-service calls |
| `quota` | Monthly API quota management per tier |
| `registry` | Service discovery with heartbeat |
| `scheduler` | Periodic background job execution |
| `sdkgen` | Go SDK code generator from API definitions |
| `security` | Security event logger for SOC/SIEM (16 event types) |
| `tenant` | Multi-tenant isolation middleware |
| `tenantctx` | Multi-tenant context isolation with validation |
| `timeutil` | Indonesian timezone support (WIB/WITA/WIT) |
| `validate` | Input validation (NIK, email, UUID, password strength) |
| `worker` | Generic worker pool for background tasks |
| `workqueue` | Priority work queue with backpressure |

### SLA & Compliance (1 package)
| Package | Purpose |
|---------|---------|
| `sla` | SLA monitoring with error budget tracking and burn rate analysis |
