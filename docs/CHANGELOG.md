# Changelog

All notable changes to GarudaPass are documented in this file.

## [0.1.0] - 2026-04-06

### Foundation (Phase 1)
- Go BFF with FAPI 2.0 OAuth2, AES-256-GCM sessions, CSRF protection
- Keycloak realm with brute force protection, TOTP, audit logging
- Kong API gateway with rate limiting, CORS, security headers
- Redis session store with encryption at rest
- PostgreSQL with connection pooling and migration runner
- Docker Compose with 24 services and resource limits
- Kubernetes manifests with HPA, NetworkPolicy, PDB, SecurityContext
- CI pipeline with matrix testing, govulncheck, Docker builds
- CD pipeline with ECR push and EKS rolling deployment

### Identity Core (Phase 2)
- NIK verification via Dukcapil with HMAC-SHA256 tokenization
- OTP service with bcrypt hashing, TTL, attempt limiting
- Field-level AES-256-GCM envelope encryption (DEK/KEK)
- Keycloak admin client for user provisioning
- Registration flow with step-by-step verification
- Data deletion endpoint (UU PDP right to erasure)
- Personal data export endpoint (UU PDP right to access)

### Verified Data (Phase 2)
- GarudaInfo consent management with field-level granularity
- Person data API filtering by active consent fields
- Consent screen data endpoint with Indonesian labels
- Privacy transparency endpoints per UU PDP
- Data lineage tracking for regulatory compliance

### Corporate Identity (Phase 3)
- AHU/SABH integration with company, officer, shareholder data
- OSS/BKPM integration with NIB cross-reference
- Triple cross-reference (AHU + OSS + Dukcapil)
- Role hierarchy enforcement (RO > Admin > User)
- Beneficial ownership (UBO) analysis per PP 13/2018

### Digital Signing (Phase 4)
- GarudaSign service with certificate lifecycle management
- In-memory ECDSA P-256 CA for dev/test (signing-sim)
- PAdES-B-LTA mock signing with hash verification
- Document upload with PDF validation and SHA-256 hashing
- Certificate revocation per RFC 5280
- Secure file storage with path traversal protection

### Developer Portal (Phase 5)
- API key generation with base62 encoding and SHA-256 hashing
- Tiered rate limiting (Free/Starter/Growth/Enterprise)
- Webhook subscriptions with HMAC-SHA256 signatures
- Webhook delivery worker with exponential backoff retry
- API key rotation with grace period
- Usage metering with daily/endpoint aggregation
- Internal key validation endpoint for Kong
- Webhook signature verification utility endpoint

### Frontend (Phase 6)
- Next.js 15 citizen portal with App Router
- Admin dashboard with system health view
- Shared shadcn/ui component library
- Turborepo monorepo for TypeScript builds
- BFF API client with server-side rendering

### Observability (Phase 7)
- Prometheus metrics with HTTP middleware
- Grafana dashboards with auto-provisioned datasources
- Jaeger distributed tracing with OTLP
- SLO-based alerting rules
- Health aggregation endpoint in BFF
- Request correlation (RequestID + CorrelationID + TraceParent)

### Audit & Compliance
- Immutable append-only audit service (PP 71/2019)
- Compliance report generator (UU PDP, PP 71/2019)
- 5-year retention policy enforcement
- Notification service (email + SMS channels)

### Shared Library (golib — 114 packages)
- **Security (10):** KMS envelope encryption, JWT ES256, permissions, fingerprinting, PII masking, digest hashing, mTLS, sanitization
- **Resilience (14):** Circuit breaker, bulkhead, singleflight, adaptive throttle, distributed lock, sliding window rate limiter, retry/fallback, request budget, backpressure/load shedding, generic connection pooling, cascading fallback, circuit-breaker HTTP transport, weighted semaphore, per-key throttle
- **Observability (13):** W3C tracing, access log percentiles, dependency checks, health graphs, K8s probes, log sampling, metrics, tags, correlation ID propagation, health aggregation, request logging, rate windows, status page
- **API Standards (10):** RFC 7807 responses, cursor pagination, content negotiation, rate limit headers, request validation (11 patterns), response writers, SQL pagination helpers
- **Data Management (10):** Transactional outbox, event sourcing, CQRS, idempotency, retention enforcement, audit trail, data lineage, seed data, exactly-once dedup, event envelope with routing/retry
- **Infrastructure (21):** Graceful shutdown, config loader, environment helpers, request IDs, tokens, batch processing, caching, events, health checks, bloom filter, context utils, generic sets, safe concurrent maps, signal-aware context
- **Domain (20):** Middleware (25+ types), OAuth2/OIDC, ABAC policy, feature flags, chaos engineering, contract testing, multi-tenancy, Indonesian timezone support
- **SLA & Compliance (1):** SLA monitoring with error budget tracking and burn rate analysis

### Infrastructure
- Terraform IaC (AWS VPC, EKS, RDS, ElastiCache, ECR, Secrets Manager)
- 16 PostgreSQL migrations
- k6 load tests (smoke, stress, soak)
- Disaster recovery plan with RPO/RTO targets
- Operations runbook
- Security policy (security.txt, CORS, CSP)
- Development seed data
- 6 OpenAPI 3.1 specifications

### Metrics
- 2,772 Go tests across all modules (race-detector verified)
- 205+ conventional commits
- 120,000+ lines of Go code
- 600+ Go files, 300+ test files
- 12 Go services + 2 Next.js apps + 4 simulators
- 114 shared library packages (zero external dependencies)
