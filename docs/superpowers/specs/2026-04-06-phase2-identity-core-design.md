# Phase 2: Identity Core — Design Specification

**Version:** 1.0  
**Date:** 2026-04-06  
**Status:** Approved  
**Depends on:** Plan 1 Foundation (completed)

---

## 1. Overview

Plan 2 builds the identity core on top of the Plan 1 foundation: Dukcapil integration for NIK-based identity verification, a Go identity service for user registration, a GarudaInfo service for consent-based verified data sharing, and a Dukcapil simulator for development/testing.

### 1.1 Scope

**In scope:**
- Dukcapil integration (NIK verification, demographic check, biometric face match)
- Dukcapil simulator for local dev and CI
- Identity service: NIK-based user registration flow
- Keycloak user creation via Admin API (Go-driven, not Java SPI)
- GarudaInfo service: field-level consent management + verified data API
- OTP verification (phone + email)
- Kafka audit events for registration and consent
- UU PDP compliance from day one

**Out of scope (deferred to later plans):**
- AHU/SABH corporate integration → Plan 3
- GarudaCorp entity verification → Plan 3
- EJBCA/EU DSS digital signing → Plan 4
- Developer portal + sandbox → Plan 5
- Biometric liveness detection (commercial SDK) → production hardening phase
- Vault integration for KEK management → Plan 5

### 1.2 Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Dukcapil integration | Hybrid: simulator for dev/CI, real API for staging | Development velocity without external dependency |
| Consent granularity | Field-level from day one | UU PDP compliance, no breaking migration later |
| Keycloak registration | External Go service + Keycloak Admin API | Avoid Java SPI, full control in Go, easier testing |
| NIK storage | Tokenized (HMAC-SHA256), never plaintext at rest | UU PDP data minimization |
| PII encryption | Per-field AES-256-GCM with envelope encryption | Granular access control, KEK rotation without re-encryption |

---

## 2. Architecture

### 2.1 New Services

```
services/
├── identity/          # Go — Registration, Dukcapil client, user management
├── garudainfo/        # Go — Consent management, verified data API
└── dukcapil-sim/      # Go — Dukcapil API simulator (dev/test only)
```

### 2.2 Service Responsibilities

**Identity Service** (`services/identity/`)
- NIK-based user registration flow (validate → Dukcapil verify → create Keycloak user)
- User profile management (CRUD with field-level encryption)
- Dukcapil client with circuit breaker, retry, and timeout
- OTP generation, delivery, and verification
- Feature flag: `DUKCAPIL_MODE=simulator|real`

**GarudaInfo Service** (`services/garudainfo/`)
- Consent management: field-level, per-purpose, per-duration
- Consent storage in PostgreSQL with immutable audit to Kafka
- Person data API: returns verified data filtered by granted consent
- Consent revocation with Kafka event notification to requesters
- Consent expiry background job

**Dukcapil Simulator** (`services/dukcapil-sim/`)
- Mock endpoints: NIK verify, demographic lookup, biometric face match
- Synthetic test data (100+ test NIKs with various scenarios)
- Configurable response delays and error rates for resilience testing
- Only deployed in dev/test environments

### 2.3 Integration Architecture

```
Browser → BFF → Identity Service → Dukcapil (sim/real)
                                  → Keycloak Admin API
                                  → PostgreSQL (encrypted PII)
                                  → Redis (OTPs)
                                  → Kafka (audit events)

Browser → BFF → GarudaInfo Service → PostgreSQL (consents)
                                    → Identity Service (user data)
                                    → Kafka (consent events)

External App → Kong → GarudaInfo API → (same as above)
```

---

## 3. Data Model

### 3.1 Users Table (Identity Service)

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    keycloak_id UUID UNIQUE NOT NULL,
    nik_token VARCHAR(64) UNIQUE NOT NULL,    -- HMAC-SHA256(nik, server_key)
    nik_masked VARCHAR(20) NOT NULL,          -- "************3456"
    name_enc BYTEA NOT NULL,                  -- AES-256-GCM encrypted
    dob_enc BYTEA NOT NULL,
    gender VARCHAR(1) NOT NULL,
    phone_hash VARCHAR(64) NOT NULL,          -- SHA-256 for lookup
    phone_enc BYTEA NOT NULL,
    email_hash VARCHAR(64) NOT NULL,
    email_enc BYTEA NOT NULL,
    address_enc BYTEA,
    auth_level SMALLINT NOT NULL DEFAULT 0,
    verification_status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    dukcapil_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_nik_token ON users(nik_token);
CREATE INDEX idx_users_phone_hash ON users(phone_hash);
CREATE INDEX idx_users_email_hash ON users(email_hash);
CREATE INDEX idx_users_keycloak_id ON users(keycloak_id);
```

### 3.2 Consents Table (GarudaInfo Service)

```sql
CREATE TABLE consents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    client_id VARCHAR(100) NOT NULL,
    client_name VARCHAR(255) NOT NULL,
    purpose VARCHAR(255) NOT NULL,
    fields JSONB NOT NULL,                    -- {"name": true, "dob": true, "address": false}
    duration_seconds BIGINT NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_consents_user_id ON consents(user_id);
CREATE INDEX idx_consents_client_id ON consents(client_id);
CREATE INDEX idx_consents_status ON consents(status);
CREATE INDEX idx_consents_expires_at ON consents(expires_at) WHERE status = 'ACTIVE';
```

### 3.3 Consent Audit Log

```sql
CREATE TABLE consent_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    consent_id UUID NOT NULL REFERENCES consents(id),
    action VARCHAR(20) NOT NULL,              -- GRANTED, REVOKED, EXPIRED, ACCESSED
    actor_id UUID,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_consent_audit_consent_id ON consent_audit_log(consent_id);
CREATE INDEX idx_consent_audit_created_at ON consent_audit_log(created_at);
```

### 3.4 Registrations Table (transient, Redis-backed)

```
Key: gpass:reg:{registration_id}
TTL: 30 minutes
Value: {
  nik_token, phone_hash, email_hash,
  phone_otp_hash, email_otp_hash,
  otp_attempts, otp_sends,
  step: "initiated|otp_verified|completed",
  created_at
}
```

---

## 4. API Design

### 4.1 Identity Service APIs (internal, BFF → service)

```
POST   /api/v1/register/initiate
  Body: { nik, phone, email }
  Response: { registration_id, otp_expires_at }
  Errors: 400 invalid_nik, 409 already_registered, 429 rate_limited

POST   /api/v1/register/verify-otp
  Body: { registration_id, phone_otp, email_otp }
  Response: { status: "otp_verified" }
  Errors: 400 invalid_otp, 410 otp_expired, 429 max_attempts

POST   /api/v1/register/verify-identity
  Body: { registration_id, password, selfie_base64 }
  Response: { user_id, keycloak_id, auth_level }
  Errors: 400 biometric_mismatch, 502 dukcapil_unavailable

GET    /api/v1/users/:id/profile
  Response: { id, nik_masked, name, dob, gender, phone, email, auth_level, verified }

PATCH  /api/v1/users/:id/profile
  Body: { phone?, email? }  (requires re-verification OTP)
  Response: { updated: true }
```

### 4.2 GarudaInfo APIs (external via Kong)

```
POST   /api/v1/garudainfo/authorize
  Body: { client_id, fields: ["name","dob","phone"], purpose, duration_days }
  Response: { consent_request_id, consent_screen_url }

GET    /api/v1/garudainfo/consent-screen?consent_request_id=xxx
  → Renders consent UI (requested fields, purpose, duration, requester)
  → User approves → redirects with authorization code

POST   /api/v1/garudainfo/token
  Body: { code, client_id, client_secret, code_verifier }
  Response: { access_token, token_type, expires_in, scope }

GET    /api/v1/garudainfo/person
  Headers: Authorization: Bearer <token>
  Response: {
    fields: {
      name: { value: "...", source: "dukcapil", last_verified: "2026-..." },
      dob: { value: "...", source: "dukcapil", last_verified: "2026-..." }
    }
  }

GET    /api/v1/garudainfo/consents
  Headers: Authorization: Bearer <session_token>
  Response: { consents: [{ id, client_name, fields, purpose, expires_at, status }] }

DELETE /api/v1/garudainfo/consents/:id
  Response: { revoked: true }
  Side effect: Kafka event → notify requesting app
```

### 4.3 Dukcapil Simulator APIs

```
POST   /api/v1/verify/nik
  Body: { nik }
  Response: { valid: true, alive: true, province: "DKI Jakarta" }

POST   /api/v1/verify/demographic
  Body: { nik, name, dob, gender }
  Response: { match: true, confidence: 0.95 }

POST   /api/v1/verify/biometric
  Body: { nik, selfie_base64 }
  Response: { match: true, score: 0.87 }
```

---

## 5. Registration Flow

```
User (Browser)          BFF (Go)           Identity Service        Dukcapil (sim/real)      Keycloak
     │                    │                      │                       │                     │
     ├─ POST /register ──►│                      │                       │                     │
     │                    ├─ POST /initiate ─────►│                       │                     │
     │                    │                      ├─ Validate NIK format  │                     │
     │                    │                      ├─ Check not registered  │                     │
     │                    │                      ├─ Send phone OTP       │                     │
     │                    │                      ├─ Send email OTP       │                     │
     │                    │◄─ { reg_id } ────────┤                       │                     │
     │◄─ Show OTP form ──┤                      │                       │                     │
     │                    │                      │                       │                     │
     ├─ Submit OTPs ─────►│                      │                       │                     │
     │                    ├─ POST /verify-otp ──►│                       │                     │
     │                    │◄─ otp_verified ──────┤                       │                     │
     │◄─ Show selfie UI ─┤                      │                       │                     │
     │                    │                      │                       │                     │
     ├─ Upload selfie ───►│                      │                       │                     │
     │                    ├─ POST /verify-id ───►│                       │                     │
     │                    │                      ├─ POST /verify/nik ───►│                     │
     │                    │                      │◄─ { valid, alive } ──┤                     │
     │                    │                      ├─ POST /biometric ────►│                     │
     │                    │                      │◄─ { score: 0.87 } ──┤                     │
     │                    │                      │                       │                     │
     │                    │                      ├─ INSERT user (encrypted PII)                │
     │                    │                      ├─ POST /admin/users ──────────────────────────►│
     │                    │                      │◄─ { keycloak_id } ───────────────────────────┤
     │                    │                      ├─ EMIT audit.registration ──► Kafka           │
     │                    │◄─ { user_id } ──────┤                       │                     │
     │◄─ Redirect login ─┤                      │                       │                     │
     │                    │                      │                       │                     │
     ├─ Login (NIK+pwd) ─►├─── Standard Keycloak OIDC flow ──────────────────────────────────►│
     │◄─ Session cookie ──┤                      │                       │                     │
```

---

## 6. Resilience Patterns

| Pattern | Where | Config |
|---------|-------|--------|
| Circuit breaker | Dukcapil client | 5 failures → open 30s → half-open |
| Retry with backoff | Dukcapil calls | 3 retries, exponential (1s, 2s, 4s) |
| Timeout | All external calls | 10s Dukcapil, 5s Keycloak Admin API |
| Rate limit | Registration endpoint | 5 registrations/IP/hour |
| Idempotency | Registration initiate | registration_id as idempotency key |
| Feature flag | Dukcapil mode | `DUKCAPIL_MODE=simulator\|real` env var |

---

## 7. Security

### 7.1 NIK Protection

```
NIK lifecycle:
1. User inputs NIK (plaintext) → HTTPS → BFF → Identity Service
2. Identity Service:
   a. Validates format (16 digits, valid province/DOB encoding)
   b. Sends plaintext to Dukcapil for verification (required by their API)
   c. nik_token = HMAC-SHA256(nik, SERVER_NIK_KEY) → stored for DB lookup
   d. nik_masked = "************" + last4 → stored for display
   e. Plaintext NIK NEVER persisted — only token + mask
3. After verification, NIK plaintext zeroed from memory
```

### 7.2 Field-Level Encryption

```
Envelope encryption per user:
  DEK (Data Encryption Key) = random AES-256 per user
  KEK (Key Encryption Key) = derived from server secret (→ Vault in Plan 5)
  
  Encrypt: field_value → AES-256-GCM(DEK, field) → store ciphertext
  Store DEK: AES-256-GCM(KEK, DEK) → store wrapped_dek in user record
  Decrypt: unwrap DEK → decrypt field
  
  KEK rotation: re-wrap all DEKs with new KEK (fast, no field re-encryption)
```

### 7.3 OTP Security

- 6-digit numeric, `crypto/rand`
- 5-minute expiry (Redis TTL)
- Max 3 verification attempts per OTP (then invalidated)
- Max 3 OTP sends per phone/email per 24 hours
- OTP stored as bcrypt hash in Redis (not plaintext)
- Rate limit: 5 registration attempts per IP per hour

### 7.4 UU PDP Consent Compliance

| Requirement | Implementation |
|-------------|----------------|
| Explicit consent | User sees exact fields, purpose, requester, duration |
| Purpose limitation | Consent tied to stated purpose, enforced on every data access |
| Withdrawable | DELETE /consents/:id → immediate revocation + Kafka event |
| Data minimization | API returns ONLY consented fields |
| Audit trail | Every action logged immutably (PostgreSQL + Kafka) |
| Retention limit | `expires_at` on consents, background expiry job |
| Data subject access | GET /consents returns all active consents |

---

## 8. Kafka Events

```
audit.registration — {
  event_id, user_id, nik_masked, status,
  verification_source: "dukcapil|simulator",
  ip_address, user_agent, timestamp
}

audit.consent — {
  event_id, consent_id, user_id, client_id,
  action: "GRANTED|REVOKED|EXPIRED|ACCESSED",
  fields, purpose, timestamp
}

events.user.lifecycle — {
  event_id, user_id,
  event: "created|verified|suspended|deleted",
  metadata, timestamp
}
```

---

## 9. Testing Strategy

| Layer | Approach |
|-------|----------|
| Identity service unit tests | Table-driven Go tests, miniredis for OTP store |
| Dukcapil client tests | httptest mock server (not simulator — unit-level) |
| Encryption tests | Round-trip encrypt/decrypt, key rotation, tamper detection |
| GarudaInfo unit tests | Consent CRUD, field filtering, expiry logic |
| Integration tests | Identity → Dukcapil simulator → Keycloak (docker-compose) |
| Consent flow tests | Full authorize → consent → token → data retrieval |
| Coverage target | ≥ 80% per package (enforced in CI) |

---

## 10. Configuration (new env vars)

```bash
# Identity Service
IDENTITY_PORT=4001
DUKCAPIL_MODE=simulator          # simulator | real
DUKCAPIL_URL=http://localhost:4002  # simulator URL or real Dukcapil endpoint
DUKCAPIL_API_KEY=                # required when DUKCAPIL_MODE=real
DUKCAPIL_TIMEOUT=10s
SERVER_NIK_KEY=<32-byte-hex>     # HMAC key for NIK tokenization
FIELD_ENCRYPTION_KEY=<32-byte-hex>  # KEK for field-level encryption
KEYCLOAK_ADMIN_URL=http://localhost:8080
KEYCLOAK_ADMIN_USER=admin
KEYCLOAK_ADMIN_PASSWORD=admin
OTP_REDIS_URL=redis://:garudapass-redis-dev@localhost:6379

# GarudaInfo Service
GARUDAINFO_PORT=4003
IDENTITY_SERVICE_URL=http://localhost:4001
GARUDAINFO_DB_URL=postgres://garudapass:garudapass@localhost:5432/garudapass

# Dukcapil Simulator
DUKCAPIL_SIM_PORT=4002
```

---

## 11. File Structure

```
services/
├── identity/
│   ├── go.mod
│   ├── main.go
│   ├── config/config.go
│   ├── handler/
│   │   ├── register.go          # Registration flow handlers
│   │   ├── register_test.go
│   │   ├── user.go              # User profile CRUD
│   │   └── user_test.go
│   ├── dukcapil/
│   │   ├── client.go            # Dukcapil HTTP client + circuit breaker
│   │   ├── client_test.go
│   │   └── types.go             # Request/response types
│   ├── crypto/
│   │   ├── nik.go               # NIK tokenization + masking
│   │   ├── nik_test.go
│   │   ├── field.go             # Field-level envelope encryption
│   │   └── field_test.go
│   ├── otp/
│   │   ├── service.go           # OTP generation, storage, verification
│   │   └── service_test.go
│   ├── keycloak/
│   │   ├── admin.go             # Keycloak Admin API client
│   │   └── admin_test.go
│   ├── store/
│   │   ├── user.go              # PostgreSQL user repository
│   │   └── user_test.go
│   └── Dockerfile
│
├── garudainfo/
│   ├── go.mod
│   ├── main.go
│   ├── config/config.go
│   ├── handler/
│   │   ├── consent.go           # Consent CRUD + consent screen
│   │   ├── consent_test.go
│   │   ├── person.go            # Verified data API
│   │   └── person_test.go
│   ├── store/
│   │   ├── consent.go           # PostgreSQL consent repository
│   │   └── consent_test.go
│   ├── audit/
│   │   ├── kafka.go             # Kafka audit event producer
│   │   └── kafka_test.go
│   └── Dockerfile
│
└── dukcapil-sim/
    ├── go.mod
    ├── main.go
    ├── handler/
    │   ├── verify.go            # NIK, demographic, biometric mock handlers
    │   └── verify_test.go
    ├── data/
    │   └── testdata.go          # 100+ synthetic NIK test records
    └── Dockerfile
```

---

## 12. Docker Compose Additions

```yaml
# Added to existing docker-compose.yml
  identity:
    build: ./services/identity
    ports: ["4001:4001"]
    env_file: .env
    depends_on: [postgres, redis, keycloak]

  garudainfo:
    build: ./services/garudainfo
    ports: ["4003:4003"]
    env_file: .env
    depends_on: [postgres, redis]

  dukcapil-sim:
    build: ./services/dukcapil-sim
    ports: ["4002:4002"]
```

---

## 13. Go Workspace Update

```
// go.work — add new services
go 1.22.2

use (
    ./apps/bff
    ./services/identity
    ./services/garudainfo
    ./services/dukcapil-sim
)
```
