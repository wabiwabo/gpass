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

### Shared Library (golib - 23 packages)
- Circuit breaker, HTTP client, retry with backoff
- AES-256-GCM PII encryption, HMAC crypto
- Input sanitization (XSS, SQL injection, path traversal)
- RBAC middleware, OAuth2 introspection, JWKS client
- Feature flags with percentage rollout
- Idempotency middleware, request throttling
- Pagination, filtering, query builder
- Runtime config with atomic values
- Prometheus metrics, audit context enrichment
- Data lineage tracking

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
- 833 Go tests across 14 modules
- 99 conventional commits
- 37,694 lines of Go code
- 293 Go files, 121 test files
- 12 Go services + 2 Next.js apps
- 23 shared library packages
