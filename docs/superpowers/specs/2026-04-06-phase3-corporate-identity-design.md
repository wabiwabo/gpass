# Phase 3: Corporate Identity — Design Specification

**Version:** 1.0  
**Date:** 2026-04-06  
**Status:** Approved  
**Depends on:** Phase 2 Identity Core (completed)

---

## 1. Overview

Phase 3 adds corporate identity features to GarudaPass: legal entity registration via AHU (Administrasi Hukum Umum) integration, OSS (Online Single Submission) enrichment for business licensing data, a GarudaCorp service for corporate registration and role management, and simulators for AHU and OSS APIs.

### 1.1 Scope

**In scope:**
- AHU integration (company search by SK number/name, officer listing, shareholder listing)
- AHU simulator for local dev and CI
- OSS integration (NIB lookup by NPWP, business license data)
- OSS simulator for local dev and CI
- GarudaCorp service: corporate entity registration, role management, entity profiles
- 3-level corporate role hierarchy: Registered Officer (RO) → Admin → User
- NIK matching via HMAC tokenization (same key as Identity Service)
- Corporate tokens with `act` claim for entity-scoped actions
- Kafka audit events for corporate actions

**Out of scope (deferred to later phases):**
- EJBCA/EU DSS digital signing → Phase 4
- Developer portal + sandbox → Phase 5
- Real AHU/OSS API integration (staging only) → production hardening
- Corporate document signing → Phase 4
- Multi-entity management dashboard → Phase 5

### 1.2 Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| AHU/OSS integration | Hybrid: simulators for dev/CI, real API for staging | Same pattern as Dukcapil — dev velocity without external dependency |
| Verification order | AHU + Dukcapil first, OSS as non-blocking enrichment | AHU provides legal entity proof; OSS enriches but is not required |
| Role hierarchy | 3-level: RO → Admin → User | RO is auto-assigned via NIK match; Admin can delegate; User has limited access |
| NIK matching | HMAC tokenization with same key as Identity Service | Consistent tokenization allows cross-service identity matching without sharing plaintext |
| Entity store | In-memory (Phase 3), PostgreSQL migration ready | Same pattern as GarudaInfo ConsentStore — interface-first, swap implementation later |

---

## 2. Architecture

### 2.1 New Services

```
services/
├── garudacorp/     # Go — Corporate registration, role management, entity profiles
├── ahu-sim/        # Go — AHU API simulator (dev/test only)
└── oss-sim/        # Go — OSS API simulator (dev/test only)
```

### 2.2 Service Responsibilities

**GarudaCorp Service** (`services/garudacorp/`)
- Corporate entity registration flow (decode SK → AHU verify → tokenize officer NIKs → NIK match → create entity + roles)
- Entity profile management with AHU officer data and OSS enrichment
- Role management: assign, list, revoke with hierarchy enforcement
- Corporate token issuance with `act` claim for entity-scoped API calls
- Feature flags: `AHU_MODE=simulator|real`, `OSS_MODE=simulator|real`

**AHU Simulator** (`services/ahu-sim/`)
- Mock endpoints: company search by SK number, company search by name, officer listing, shareholder listing
- Synthetic test data (10+ companies: PT, CV, Yayasan with officers whose NIKs match dukcapil-sim test data)
- Only deployed in dev/test environments

**OSS Simulator** (`services/oss-sim/`)
- Mock endpoints: NIB search by NPWP
- Synthetic test data linked to AHU companies by NPWP
- Only deployed in dev/test environments

### 2.3 Integration Architecture

```
Browser → BFF → GarudaCorp → AHU (sim/real)
                            → OSS (sim/real)
                            → Identity Service (NIK token lookup)
                            → Kafka (audit events)

Corporate Registration Flow:
  1. User provides SK number
  2. GarudaCorp calls AHU → get company + officers
  3. GarudaCorp tokenizes each officer NIK with SERVER_NIK_KEY
  4. GarudaCorp matches caller's nik_token against officer list
  5. If match found → create entity + auto-assign RO role
  6. Background: call OSS for NIB enrichment (non-blocking)
```

---

## 3. Data Model

### 3.1 Entities Table

```sql
-- infrastructure/db/migrations/003_create_entities.sql
CREATE TABLE entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ahu_sk_number VARCHAR(100) UNIQUE NOT NULL,
    name VARCHAR(500) NOT NULL,
    entity_type VARCHAR(20) NOT NULL,          -- PT, CV, YAYASAN, KOPERASI
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    npwp VARCHAR(20),
    address TEXT,
    capital_authorized BIGINT,                 -- in IDR
    capital_paid BIGINT,                       -- in IDR
    ahu_verified_at TIMESTAMPTZ,
    oss_nib VARCHAR(20),
    oss_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entities_ahu_sk ON entities(ahu_sk_number);
CREATE INDEX idx_entities_npwp ON entities(npwp);
CREATE INDEX idx_entities_status ON entities(status);
```

### 3.2 Entity Officers Table

```sql
-- infrastructure/db/migrations/004_create_entity_officers.sql
CREATE TABLE entity_officers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id UUID NOT NULL REFERENCES entities(id),
    user_id UUID REFERENCES users(id),         -- nullable until user registers
    nik_token VARCHAR(64) NOT NULL,            -- HMAC-SHA256(nik, SERVER_NIK_KEY)
    name VARCHAR(255) NOT NULL,
    position VARCHAR(50) NOT NULL,             -- DIREKTUR_UTAMA, KOMISARIS, etc.
    appointment_date DATE,
    source VARCHAR(20) NOT NULL DEFAULT 'AHU', -- AHU, MANUAL
    verified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entity_officers_entity_id ON entity_officers(entity_id);
CREATE INDEX idx_entity_officers_nik_token ON entity_officers(nik_token);
CREATE INDEX idx_entity_officers_user_id ON entity_officers(user_id);
```

### 3.3 Entity Roles Table

```sql
-- infrastructure/db/migrations/005_create_entity_roles.sql
CREATE TABLE entity_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id UUID NOT NULL REFERENCES entities(id),
    user_id UUID NOT NULL REFERENCES users(id),
    role VARCHAR(20) NOT NULL,                 -- RO, ADMIN, USER
    granted_by UUID REFERENCES users(id),      -- who granted this role
    service_access JSONB,                      -- {"garudainfo": true, "signing": false}
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entity_roles_entity_id ON entity_roles(entity_id);
CREATE INDEX idx_entity_roles_user_id ON entity_roles(user_id);
CREATE INDEX idx_entity_roles_status ON entity_roles(status);
CREATE UNIQUE INDEX idx_entity_roles_active_unique
    ON entity_roles(entity_id, user_id) WHERE status = 'ACTIVE';
```

### 3.4 Entity Shareholders Table

```sql
-- infrastructure/db/migrations/006_create_entity_shareholders.sql
CREATE TABLE entity_shareholders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id UUID NOT NULL REFERENCES entities(id),
    name VARCHAR(255) NOT NULL,
    share_type VARCHAR(50) NOT NULL,           -- SAHAM_BIASA, SAHAM_PREFEREN
    shares BIGINT NOT NULL,
    percentage DECIMAL(5,2) NOT NULL,
    source VARCHAR(20) NOT NULL DEFAULT 'AHU',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entity_shareholders_entity_id ON entity_shareholders(entity_id);
```

---

## 4. API Design

### 4.1 GarudaCorp APIs (internal, BFF → service)

```
POST   /api/v1/corp/register
  Headers: X-User-ID: <user_id>, X-NIK-Token: <nik_token>
  Body: { sk_number: "AHU-0012345.AH.01.01.TAHUN2024" }
  Response: { entity_id, entity_name, role: "RO" }
  Errors: 400 invalid_sk, 404 company_not_found, 403 nik_not_officer, 409 already_registered

GET    /api/v1/corp/entities/{id}
  Headers: X-User-ID: <user_id>
  Response: {
    id, name, entity_type, status, npwp, address,
    capital_authorized, capital_paid,
    ahu_verified_at, oss_nib, oss_verified_at,
    officers: [{ id, name, position, verified }],
    shareholders: [{ name, share_type, shares, percentage }]
  }
  Errors: 403 not_authorized, 404 entity_not_found

POST   /api/v1/corp/entities/{id}/roles
  Headers: X-User-ID: <user_id>
  Body: { target_user_id, role: "ADMIN"|"USER", service_access: {...} }
  Response: { role_id, role, status: "ACTIVE" }
  Errors: 403 insufficient_role, 404 entity_not_found, 409 role_exists

GET    /api/v1/corp/entities/{id}/roles
  Headers: X-User-ID: <user_id>
  Response: { roles: [{ id, user_id, role, status, granted_at, granted_by }] }

DELETE /api/v1/corp/entities/{id}/roles/{role_id}
  Headers: X-User-ID: <user_id>
  Response: { revoked: true }
  Errors: 403 insufficient_role, 404 role_not_found

POST   /api/v1/corp/entities/{id}/token
  Headers: X-User-ID: <user_id>
  Response: { corporate_token, expires_in }
  Token contains: { sub: user_id, act: { entity_id, role }, exp, iat }
```

### 4.2 AHU Simulator APIs

```
GET    /api/v1/ahu/company?sk_number=AHU-0012345.AH.01.01.TAHUN2024
  Response: {
    sk_number, name, entity_type, npwp, address,
    capital_authorized, capital_paid,
    established_date
  }
  Errors: 404 not_found

GET    /api/v1/ahu/company/search?name=PT+Maju+Jaya
  Response: { companies: [{ sk_number, name, entity_type }] }

GET    /api/v1/ahu/company/{sk_number}/officers
  Response: { officers: [{ nik, name, position, appointment_date }] }
  Note: NIK returned in plaintext from AHU (as the real API would)

GET    /api/v1/ahu/company/{sk_number}/shareholders
  Response: { shareholders: [{ name, share_type, shares, percentage }] }

GET    /health
  Response: { status: "ok", service: "ahu-simulator" }
```

### 4.3 OSS Simulator APIs

```
GET    /api/v1/oss/nib?npwp=01.234.567.8-012.000
  Response: {
    nib, npwp, company_name, business_type,
    kbli_codes: [{ code, description }],
    issued_date, status
  }
  Errors: 404 not_found

GET    /health
  Response: { status: "ok", service: "oss-simulator" }
```

---

## 5. Corporate Registration Flow

```
User (Browser)          BFF (Go)           GarudaCorp            AHU (sim/real)        OSS (sim/real)
     │                    │                    │                      │                     │
     ├─ POST /register ──►│                    │                      │                     │
     │   { sk_number }    ├─ POST /corp/reg ──►│                      │                     │
     │                    │  +X-User-ID        │                      │                     │
     │                    │  +X-NIK-Token       ├─ GET /company?sk ───►│                     │
     │                    │                    │◄─ { company } ──────┤                     │
     │                    │                    │                      │                     │
     │                    │                    ├─ GET /officers ─────►│                     │
     │                    │                    │◄─ [officers+NIKs] ──┤                     │
     │                    │                    │                      │                     │
     │                    │                    ├─ For each officer:    │                     │
     │                    │                    │  HMAC(nik, KEY) ──►  │                     │
     │                    │                    │  → nik_token          │                     │
     │                    │                    │                      │                     │
     │                    │                    ├─ Match caller's       │                     │
     │                    │                    │  nik_token against    │                     │
     │                    │                    │  officer nik_tokens   │                     │
     │                    │                    │                      │                     │
     │                    │                    ├─ CREATE entity        │                     │
     │                    │                    ├─ CREATE officers      │                     │
     │                    │                    ├─ CREATE RO role       │                     │
     │                    │                    │                      │                     │
     │                    │                    ├─ (async) GET /nib ────────────────────────►│
     │                    │                    │◄─ { nib, kbli } ─────────────────────────┤
     │                    │                    ├─ UPDATE entity.oss_*  │                     │
     │                    │                    │                      │                     │
     │                    │                    ├─ EMIT audit.corporate ──► Kafka             │
     │                    │◄─ { entity_id } ──┤                      │                     │
     │◄─ Show entity ────┤                    │                      │                     │
```

---

## 6. Security

### 6.1 NIK Matching Without Plaintext

```
NIK matching flow:
1. AHU returns officer NIKs in plaintext (as the real API does)
2. GarudaCorp tokenizes each officer NIK: HMAC-SHA256(nik, SERVER_NIK_KEY)
3. Plaintext NIK is zeroed from memory immediately after tokenization
4. Caller's nik_token (passed via X-NIK-Token header) is compared against officer tokens
5. If match found → caller is a verified officer → auto-assign RO role
6. Only nik_token is persisted in entity_officers — never plaintext NIK
7. SERVER_NIK_KEY is the SAME key used by Identity Service — ensures token consistency
```

### 6.2 Role Hierarchy Enforcement

```
Role hierarchy: RO > ADMIN > USER

Rules:
- RO can assign ADMIN or USER roles
- ADMIN can assign USER roles only
- USER cannot assign any roles
- RO can revoke ADMIN or USER roles
- ADMIN can revoke USER roles only
- RO role is auto-assigned via NIK match — cannot be manually assigned
- Each user can have at most one active role per entity (enforced by unique index)
```

### 6.3 Corporate Tokens

```
Corporate token (JWT) structure:
{
  "sub": "<user_id>",
  "act": {
    "entity_id": "<entity_id>",
    "role": "RO|ADMIN|USER"
  },
  "exp": <expiry>,
  "iat": <issued_at>,
  "iss": "garudacorp"
}

Usage:
- Downstream services check the `act` claim to verify entity-scoped authorization
- Token expires in 1 hour
- Only users with an active role on the entity can request a corporate token
```

### 6.4 Audit Trail

- Every corporate action is logged to Kafka `audit.corporate` topic
- Actions: ENTITY_REGISTERED, ROLE_ASSIGNED, ROLE_REVOKED, ENTITY_VIEWED, TOKEN_ISSUED
- Each event includes: actor_id, entity_id, action, metadata, timestamp
- Immutable — no deletes or updates to audit records

---

## 7. Kafka Events

```
audit.corporate — {
  event_id: UUID,
  entity_id: UUID,
  actor_id: UUID,
  action: "ENTITY_REGISTERED|ROLE_ASSIGNED|ROLE_REVOKED|ENTITY_VIEWED|TOKEN_ISSUED",
  metadata: {
    // varies by action
    entity_name: "...",
    role: "RO|ADMIN|USER",
    target_user_id: "...",
    sk_number: "..."
  },
  timestamp: "2026-04-06T..."
}
```

---

## 8. Testing Strategy

| Layer | Approach |
|-------|----------|
| AHU simulator tests | Table-driven Go tests with httptest |
| OSS simulator tests | Table-driven Go tests with httptest |
| AHU client tests | httptest mock server (not simulator — unit-level) |
| OSS client tests | httptest mock server (not simulator — unit-level) |
| Entity store tests | In-memory store CRUD, role hierarchy enforcement |
| Registration handler tests | Mock AHU/OSS clients + mock store, verify NIK tokenization flow |
| Role handler tests | Mock store, verify hierarchy enforcement |
| Entity profile handler tests | Mock store + mock OSS client for enrichment |
| Integration tests | GarudaCorp → AHU-sim → OSS-sim (docker-compose) |
| Coverage target | >= 80% per package (enforced in CI) |

---

## 9. Configuration (new env vars)

```bash
# GarudaCorp Service
GARUDACORP_PORT=4006
AHU_MODE=simulator               # simulator | real
AHU_URL=http://localhost:4004     # simulator URL or real AHU endpoint
AHU_API_KEY=                      # required when AHU_MODE=real
AHU_TIMEOUT=10s
OSS_MODE=simulator                # simulator | real
OSS_URL=http://localhost:4005     # simulator URL or real OSS endpoint
OSS_API_KEY=                      # required when OSS_MODE=real
OSS_TIMEOUT=10s
SERVER_NIK_KEY=<same-as-identity> # MUST be same key as Identity Service
CORPORATE_TOKEN_SECRET=<32-byte-hex>  # HMAC key for corporate JWT signing

# AHU Simulator
AHU_SIM_PORT=4004

# OSS Simulator
OSS_SIM_PORT=4005
```

---

## 10. File Structure

```
services/
├── garudacorp/
│   ├── go.mod
│   ├── main.go
│   ├── Dockerfile
│   ├── config/
│   │   ├── config.go                          # Environment config with validation
│   │   └── config_test.go
│   ├── ahu/
│   │   ├── client.go                          # AHU HTTP client + circuit breaker
│   │   ├── client_test.go
│   │   └── types.go                           # AHU request/response types
│   ├── oss/
│   │   ├── client.go                          # OSS HTTP client + circuit breaker
│   │   ├── client_test.go
│   │   └── types.go                           # OSS request/response types
│   ├── store/
│   │   ├── entity.go                          # Entity + officer + shareholder store
│   │   ├── entity_test.go
│   │   ├── role.go                            # Role assignment store
│   │   └── role_test.go
│   ├── handler/
│   │   ├── register.go                        # Corporate registration handler
│   │   ├── register_test.go
│   │   ├── entity.go                          # Entity profile handler
│   │   ├── entity_test.go
│   │   ├── role.go                            # Role management handlers
│   │   └── role_test.go
│   └── Dockerfile
│
├── ahu-sim/
│   ├── go.mod
│   ├── main.go
│   ├── Dockerfile
│   ├── data/
│   │   └── testdata.go                        # Synthetic company data (10+ companies)
│   └── handler/
│       ├── company.go                         # Company search + officers + shareholders
│       └── company_test.go
│
└── oss-sim/
    ├── go.mod
    ├── main.go
    ├── Dockerfile
    ├── data/
    │   └── testdata.go                        # Synthetic NIB data linked by NPWP
    └── handler/
        ├── nib.go                             # NIB search handler
        └── nib_test.go
```

---

## 11. Docker Compose Additions

```yaml
# Added to existing docker-compose.yml
  ahu-sim:
    build: ./services/ahu-sim
    restart: unless-stopped
    ports:
      - "4004:4004"
    environment:
      AHU_SIM_PORT: "4004"
    deploy:
      resources:
        limits:
          memory: 128M
          cpus: "0.25"
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    networks:
      - gpass-network

  oss-sim:
    build: ./services/oss-sim
    restart: unless-stopped
    ports:
      - "4005:4005"
    environment:
      OSS_SIM_PORT: "4005"
    deploy:
      resources:
        limits:
          memory: 128M
          cpus: "0.25"
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    networks:
      - gpass-network

  garudacorp:
    build: ./services/garudacorp
    restart: unless-stopped
    ports:
      - "4006:4006"
    env_file: .env
    depends_on:
      ahu-sim:
        condition: service_started
      oss-sim:
        condition: service_started
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
// go.work — add new services
go 1.25.0

use (
    ./apps/bff
    ./services/dukcapil-sim
    ./services/garudainfo
    ./services/identity
    ./services/garudacorp
    ./services/ahu-sim
    ./services/oss-sim
)
```
