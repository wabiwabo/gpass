# Phase 4: Digital Signing — Design Specification

**Version:** 1.0  
**Date:** 2026-04-06  
**Status:** Draft  
**Depends on:** Phase 2 Identity Core (completed), Phase 3 Corporate Identity (in progress)

---

## 1. Overview

Phase 4 adds digital signing to GarudaPass: a GarudaSign Go service that orchestrates certificate issuance and PAdES-B-LTA PDF signing, backed by EJBCA (Certificate Authority) and EU DSS (signing engine) in staging/production, with a Go signing-sim for local development and CI.

### 1.1 Scope

**In scope:**
- GarudaSign service: certificate request, document upload, hash-based signing, signed PDF retrieval
- Signing simulator for local dev and CI (mocks EJBCA certificate issuance + EU DSS PAdES signing)
- 2-tier CA hierarchy for MVP (Root CA → Issuing CA)
- PAdES-B-LTA PDF signatures (most common use case)
- Individual signing only (single signer per document)
- Step-up auth simulation via existing L2 auth (PKCE session)
- EJBCA Community Edition with file-based keystore (no HSM)
- Kafka audit events for all signing actions
- Signing certificate lifecycle management

**Out of scope (deferred to later phases):**
- Multi-party / corporate signing → Phase 9
- HSM integration (CloudHSM) → production hardening
- 3-tier CA hierarchy (Root → Sub → Issuing) → production hardening
- BSrE integration for government-grade certificates → future
- XAdES, CAdES, JAdES signature formats → future
- EJBCA Enterprise Edition → production
- Real biometric/device key step-up to L4 → production hardening
- Mobile signing flow → Phase 6 (Flutter app)
- Timestamp Authority (RFC 3161) as separate service → production hardening

### 1.2 Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Certificate Authority | EJBCA Community Edition | Free, Common Criteria certified lineage, sufficient for MVP |
| Signing engine | EU DSS | Gold standard for AdES signatures, eIDAS compliant, LGPL |
| CA hierarchy | 2-tier (Root CA → Issuing CA) for MVP | Sub CA adds operational complexity without MVP value |
| Signing service | Go | Consistent with all other GarudaPass services |
| Signature format | PAdES-B-LTA | Long-term archival, most common use case (PDF signing) |
| Key storage (MVP) | File-based PKCS12 keystore | HSM deferred to production — avoids CloudHSM cost during dev |
| Step-up auth (MVP) | Simulated via L2 (PKCE session) | Real L4 (biometric + device key) requires mobile app infrastructure |
| Dev/test integration | Go signing-sim mocking EJBCA + DSS APIs | Avoid running Java services locally; fast CI |
| Signing scope | Individual only | Multi-party signing is architecturally different; deferred to Phase 9 |

---

## 2. Architecture

### 2.1 Environment-Based Topology

```
DEV/TEST:
  Browser → BFF → GarudaSign (Go) → signing-sim (Go, mocks EJBCA + DSS)
                                   → PostgreSQL (certificates, requests, signed docs metadata)
                                   → Kafka (audit events)

STAGING/PROD:
  Browser → BFF → GarudaSign (Go) → EJBCA (Java, certificate issuance)
                                   → EU DSS (Java, PAdES signing)
                                   → PostgreSQL
                                   → Kafka
```

### 2.2 New Services

```
services/
├── garudasign/      # Go — Signing orchestration service
└── signing-sim/     # Go — Mock EJBCA + EU DSS for dev/test
```

### 2.3 Service Responsibilities

**GarudaSign Service** (`services/garudasign/`)
- Certificate request flow (validate user → step-up auth check → request cert from EJBCA/sim)
- Certificate lifecycle management (active, expired, revoked)
- Document upload, hash computation (SHA-256), and temporary storage
- Signing orchestration: lookup certificate → send hash to DSS/sim → receive signed PDF
- Signed document storage and retrieval
- Feature flag: `SIGNING_MODE=simulator|real`

**Signing Simulator** (`services/signing-sim/`)
- Mock EJBCA endpoint: issue X.509 certificate (self-signed, valid structure but not trusted)
- Mock EU DSS endpoint: accept document hash + certificate, return mock-signed PDF with PAdES-B-LTA structure
- Deterministic responses for predictable testing
- Only deployed in dev/test environments

### 2.4 Integration Architecture

```
Browser → BFF → GarudaSign → signing-sim (dev) / EJBCA + EU DSS (staging/prod)
                            → Identity Service (user lookup, auth level check)
                            → PostgreSQL (certificates, signing requests, signed docs)
                            → Kafka (audit events)
                            → File storage (uploaded + signed PDFs, local disk for MVP)
```

---

## 3. Data Model

### 3.1 Signing Certificates Table

```sql
-- infrastructure/db/migrations/007_create_signing_certificates.sql
CREATE TABLE signing_certificates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    serial_number VARCHAR(64) UNIQUE NOT NULL,     -- hex-encoded certificate serial
    issuer_dn VARCHAR(500) NOT NULL,               -- issuer distinguished name
    subject_dn VARCHAR(500) NOT NULL,              -- subject distinguished name
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',  -- ACTIVE, EXPIRED, REVOKED
    valid_from TIMESTAMPTZ NOT NULL,
    valid_to TIMESTAMPTZ NOT NULL,
    certificate_pem TEXT NOT NULL,                  -- PEM-encoded X.509 certificate
    fingerprint_sha256 VARCHAR(64) NOT NULL,        -- SHA-256 fingerprint for lookup
    revoked_at TIMESTAMPTZ,
    revocation_reason VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_signing_certs_user_id ON signing_certificates(user_id);
CREATE INDEX idx_signing_certs_status ON signing_certificates(status);
CREATE INDEX idx_signing_certs_serial ON signing_certificates(serial_number);
CREATE INDEX idx_signing_certs_fingerprint ON signing_certificates(fingerprint_sha256);
CREATE INDEX idx_signing_certs_valid_to ON signing_certificates(valid_to) WHERE status = 'ACTIVE';
```

### 3.2 Signing Requests Table

```sql
-- infrastructure/db/migrations/008_create_signing_requests.sql
CREATE TABLE signing_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    certificate_id UUID REFERENCES signing_certificates(id),  -- set when signing executes
    document_name VARCHAR(255) NOT NULL,
    document_size BIGINT NOT NULL,                             -- bytes
    document_hash VARCHAR(64) NOT NULL,                        -- SHA-256 of uploaded document
    document_path VARCHAR(500) NOT NULL,                       -- file storage path
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',             -- PENDING, SIGNING, COMPLETED, FAILED, EXPIRED
    error_message TEXT,
    expires_at TIMESTAMPTZ NOT NULL,                           -- request expiry (30 min from upload)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_signing_requests_user_id ON signing_requests(user_id);
CREATE INDEX idx_signing_requests_status ON signing_requests(status);
CREATE INDEX idx_signing_requests_expires_at ON signing_requests(expires_at) WHERE status = 'PENDING';
```

### 3.3 Signed Documents Table

```sql
-- infrastructure/db/migrations/009_create_signed_documents.sql
CREATE TABLE signed_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID UNIQUE NOT NULL REFERENCES signing_requests(id),
    certificate_id UUID NOT NULL REFERENCES signing_certificates(id),
    signed_hash VARCHAR(64) NOT NULL,              -- SHA-256 of signed document
    signed_path VARCHAR(500) NOT NULL,             -- file storage path for signed PDF
    signed_size BIGINT NOT NULL,                   -- bytes
    pades_level VARCHAR(20) NOT NULL,              -- B_LTA
    signature_timestamp TIMESTAMPTZ NOT NULL,       -- when signature was applied
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_signed_docs_request_id ON signed_documents(request_id);
CREATE INDEX idx_signed_docs_certificate_id ON signed_documents(certificate_id);
```

---

## 4. API Design

### 4.1 GarudaSign APIs (internal, BFF -> service)

```
POST   /api/v1/sign/certificates/request
  Headers: X-User-ID: <user_id>
  Body: { common_name?: "User's display name" }
  Response: { certificate_id, serial_number, subject_dn, valid_from, valid_to, status }
  Errors: 400 invalid_request, 409 active_cert_exists, 502 ca_unavailable

GET    /api/v1/sign/certificates
  Headers: X-User-ID: <user_id>
  Query: ?status=ACTIVE
  Response: {
    certificates: [{
      id, serial_number, subject_dn, issuer_dn,
      status, valid_from, valid_to, fingerprint_sha256
    }]
  }

POST   /api/v1/sign/documents
  Headers: X-User-ID: <user_id>
  Body: multipart/form-data { file: <pdf_file> }
  Response: { request_id, document_name, document_hash, status: "PENDING", expires_at }
  Errors: 400 invalid_file_type, 413 file_too_large (max 10MB)

POST   /api/v1/sign/documents/:id/sign
  Headers: X-User-ID: <user_id>
  Body: { certificate_id: "<uuid>" }
  Response: {
    request_id, status: "COMPLETED",
    signed_document: { id, signed_hash, pades_level, signature_timestamp }
  }
  Errors: 400 invalid_certificate, 403 not_owner, 404 request_not_found,
          410 request_expired, 409 already_signed, 502 signing_engine_unavailable

GET    /api/v1/sign/documents/:id
  Headers: X-User-ID: <user_id>
  Response: {
    id, document_name, document_hash, status, created_at,
    signed_document?: { id, signed_hash, pades_level, signature_timestamp }
  }
  Errors: 403 not_owner, 404 not_found

GET    /api/v1/sign/documents/:id/download
  Headers: X-User-ID: <user_id>
  Response: application/pdf (signed PDF binary)
  Errors: 403 not_owner, 404 not_found, 409 not_yet_signed
```

### 4.2 Signing Simulator APIs

```
POST   /certificates/issue
  Body: {
    subject_cn: "John Doe",
    subject_uid: "<user_id>",
    validity_days: 365
  }
  Response: {
    serial_number: "0A1B2C...",
    certificate_pem: "-----BEGIN CERTIFICATE-----\n...",
    issuer_dn: "CN=GarudaPass Dev Root CA,O=GarudaPass,C=ID",
    subject_dn: "CN=John Doe,UID=<user_id>,O=GarudaPass,C=ID",
    valid_from: "2026-04-06T...",
    valid_to: "2027-04-06T...",
    fingerprint_sha256: "AB12CD..."
  }

POST   /sign/pades
  Body: {
    document_base64: "<base64-encoded PDF>",
    certificate_pem: "-----BEGIN CERTIFICATE-----\n...",
    signature_level: "PAdES_BASELINE_LTA"
  }
  Response: {
    signed_document_base64: "<base64-encoded signed PDF>",
    signature_timestamp: "2026-04-06T...",
    pades_level: "B_LTA"
  }

GET    /health
  Response: { status: "ok", service: "signing-simulator" }
```

---

## 5. Signing Flow

```
User (Browser)          BFF (Go)           GarudaSign           signing-sim / EJBCA+DSS    Identity Svc
     |                    |                    |                      |                        |
     |  [Certificate Request — one-time setup]                       |                        |
     |                    |                    |                      |                        |
     +- POST /certs/req ->|                    |                      |                        |
     |                    +- POST /certs/req ->|                      |                        |
     |                    |                    +- GET user auth lvl ->+----------------------->|
     |                    |                    |<- auth_level: L2 ----+------------------------+
     |                    |                    |  (MVP: accept L2)    |                        |
     |                    |                    +- POST /certs/issue ->|                        |
     |                    |                    |<- { cert_pem, ... } -+                        |
     |                    |                    +- INSERT certificate  |                        |
     |                    |                    +- EMIT audit.signing  |                        |
     |                    |<- { cert_id } -----+                      |                        |
     |<- Show cert -------+                    |                      |                        |
     |                    |                    |                      |                        |
     |  [Document Upload]                      |                      |                        |
     |                    |                    |                      |                        |
     +- POST /documents ->|                    |                      |                        |
     |  (multipart PDF)   +- POST /documents ->|                      |                        |
     |                    |                    +- Validate PDF        |                        |
     |                    |                    +- SHA-256(document)   |                        |
     |                    |                    +- Store PDF to disk   |                        |
     |                    |                    +- INSERT signing_req  |                        |
     |                    |                    +- EMIT audit.signing  |                        |
     |                    |<- { request_id } --+                      |                        |
     |<- Show confirm ----+                    |                      |                        |
     |                    |                    |                      |                        |
     |  [Execute Signing]                      |                      |                        |
     |                    |                    |                      |                        |
     +- POST /sign ------>|                    |                      |                        |
     |  { cert_id }       +- POST /:id/sign ->|                      |                        |
     |                    |                    +- Verify cert ACTIVE  |                        |
     |                    |                    +- Verify cert belongs |                        |
     |                    |                    |  to user             |                        |
     |                    |                    +- Read PDF from disk  |                        |
     |                    |                    +- POST /sign/pades -->|                        |
     |                    |                    |  { doc, cert, level }|                        |
     |                    |                    |<- { signed_doc } ----+                        |
     |                    |                    +- Store signed PDF    |                        |
     |                    |                    +- INSERT signed_doc   |                        |
     |                    |                    +- UPDATE request=DONE |                        |
     |                    |                    +- EMIT audit.signing  |                        |
     |                    |<- { signed_doc } --+                      |                        |
     |<- Show download ----+                    |                      |                        |
     |                    |                    |                      |                        |
     |  [Download Signed PDF]                  |                      |                        |
     |                    |                    |                      |                        |
     +- GET /download --->|                    |                      |                        |
     |                    +- GET /:id/dl ----->|                      |                        |
     |                    |                    +- Read signed PDF     |                        |
     |                    |<- PDF binary ------+                      |                        |
     |<- PDF download ----+                    |                      |                        |
```

---

## 6. Security

### 6.1 Private Key Protection

```
Key lifecycle:
1. Private key generated INSIDE EJBCA (or signing-sim for dev)
2. Private key NEVER leaves EJBCA / HSM boundary
3. GarudaSign sends document HASH to signing engine — not full document
4. Signing engine signs hash with private key internally
5. Only the signed document (with embedded signature) is returned
6. GarudaSign never holds or sees the private key

MVP (signing-sim):
- Self-signed CA key generated at simulator startup
- Stored in memory only (not persisted)
- Per-user keys generated on certificate request, held in memory
- Suitable for dev/test only — NOT for production
```

### 6.2 Document Integrity

```
Hash chain:
1. Upload: document_hash = SHA-256(original_pdf) — stored in signing_requests
2. Sign: hash is sent to signing engine (not full document content over wire for real EJBCA)
3. Complete: signed_hash = SHA-256(signed_pdf) — stored in signed_documents
4. Verify: any party can verify by checking PAdES signature in the signed PDF

Tamper detection:
- document_hash in DB must match SHA-256(file on disk)
- If mismatch detected, signing is refused
```

### 6.3 Step-Up Authentication

```
MVP (Phase 4):
- GarudaSign checks user's auth_level via Identity Service
- Accepts L2 (PKCE session with OTP/passkey) as sufficient for MVP
- Logs the auth_level used in audit trail

Production target:
- Require L4 (biometric + device key) before signing
- CIBA push notification for step-up
- Device Secure Enclave for key confirmation
```

### 6.4 Audit Trail

- Every signing action logged to Kafka `audit.signing` topic
- Actions: CERT_REQUESTED, CERT_ISSUED, CERT_REVOKED, DOC_UPLOADED, DOC_SIGNED, DOC_DOWNLOADED, SIGN_FAILED
- Each event includes: actor_id, certificate_id, request_id, action, metadata, timestamp
- Immutable — no deletes or updates to audit records
- 5-year retention per PP 71/2019

---

## 7. Kafka Events

```
audit.signing — {
  event_id: UUID,
  user_id: UUID,
  action: "CERT_REQUESTED|CERT_ISSUED|CERT_REVOKED|DOC_UPLOADED|DOC_SIGNED|DOC_DOWNLOADED|SIGN_FAILED",
  metadata: {
    // varies by action
    certificate_id: "...",
    serial_number: "...",
    request_id: "...",
    document_name: "...",
    document_hash: "...",
    pades_level: "B_LTA",
    auth_level: 2,
    error: "..."       // only for SIGN_FAILED
  },
  timestamp: "2026-04-06T..."
}
```

---

## 8. Testing Strategy

| Layer | Approach |
|-------|----------|
| Signing-sim tests | Table-driven Go tests: certificate issuance, PAdES mock signing |
| GarudaSign handler tests | httptest mock server for signing-sim, mock stores |
| Certificate store tests | CRUD, status transitions (ACTIVE → EXPIRED, ACTIVE → REVOKED) |
| Signing request store tests | CRUD, expiry logic, status transitions |
| Document hash tests | SHA-256 round-trip, tamper detection |
| File upload tests | PDF validation, size limits, path sanitization |
| Auth level check tests | Verify L2 accepted (MVP), L4 required (prod config) |
| Integration tests | GarudaSign → signing-sim → PostgreSQL (docker-compose) |
| End-to-end flow test | Upload PDF → request cert → sign → download → verify hash chain |
| Coverage target | >= 80% per package (enforced in CI) |

---

## 9. Configuration (new env vars)

```bash
# GarudaSign Service
GARUDASIGN_PORT=4007
SIGNING_MODE=simulator                      # simulator | real
SIGNING_SIM_URL=http://localhost:4008        # signing-sim URL
EJBCA_URL=                                  # required when SIGNING_MODE=real
EJBCA_CLIENT_CERT=                          # mTLS client cert for EJBCA
EJBCA_CLIENT_KEY=                           # mTLS client key for EJBCA
DSS_URL=                                    # EU DSS URL, required when SIGNING_MODE=real
IDENTITY_SERVICE_URL=http://localhost:4001   # for auth level checks
DOCUMENT_STORAGE_PATH=/data/signing          # local file storage for PDFs
DOCUMENT_MAX_SIZE_MB=10                      # max upload size
SIGNING_REQUEST_TTL=30m                      # request expiry
CERT_VALIDITY_DAYS=365                       # default certificate validity
GARUDASIGN_DB_URL=postgres://garudapass:garudapass@localhost:5432/garudapass

# Signing Simulator
SIGNING_SIM_PORT=4008
```

---

## 10. File Structure

```
services/
├── garudasign/
│   ├── go.mod
│   ├── main.go
│   ├── Dockerfile
│   ├── config/
│   │   ├── config.go                          # Environment config with validation
│   │   └── config_test.go
│   ├── signing/
│   │   ├── client.go                          # EJBCA/DSS HTTP client (or signing-sim)
│   │   ├── client_test.go
│   │   └── types.go                           # Certificate + signing request/response types
│   ├── store/
│   │   ├── certificate.go                     # Signing certificate CRUD + lifecycle
│   │   ├── certificate_test.go
│   │   ├── request.go                         # Signing request CRUD + expiry
│   │   ├── request_test.go
│   │   ├── document.go                        # Signed document CRUD
│   │   └── document_test.go
│   ├── handler/
│   │   ├── certificate.go                     # Certificate request + list handlers
│   │   ├── certificate_test.go
│   │   ├── document.go                        # Document upload, sign, status, download
│   │   └── document_test.go
│   ├── hash/
│   │   ├── sha256.go                          # Document hash computation + verification
│   │   └── sha256_test.go
│   ├── storage/
│   │   ├── file.go                            # Local file storage for PDFs
│   │   └── file_test.go
│   ├── audit/
│   │   ├── kafka.go                           # Kafka audit event producer
│   │   └── kafka_test.go
│   └── Dockerfile
│
└── signing-sim/
    ├── go.mod
    ├── main.go
    ├── Dockerfile
    ├── ca/
    │   ├── ca.go                              # In-memory self-signed CA + cert issuance
    │   └── ca_test.go
    ├── pades/
    │   ├── mock.go                            # Mock PAdES-B-LTA signing (embeds fake sig)
    │   └── mock_test.go
    └── handler/
        ├── certificate.go                     # POST /certificates/issue
        ├── certificate_test.go
        ├── sign.go                            # POST /sign/pades
        └── sign_test.go
```

---

## 11. Docker Compose Additions

```yaml
# Added to existing docker-compose.yml
  signing-sim:
    build: ./services/signing-sim
    restart: unless-stopped
    ports:
      - "4008:4008"
    environment:
      SIGNING_SIM_PORT: "4008"
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

  garudasign:
    build: ./services/garudasign
    restart: unless-stopped
    ports:
      - "4007:4007"
    env_file: .env
    volumes:
      - signing-data:/data/signing
    depends_on:
      signing-sim:
        condition: service_started
      postgres:
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

volumes:
  signing-data:
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
    ./services/garudasign
    ./services/signing-sim
)
```
