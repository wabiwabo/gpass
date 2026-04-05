# GarudaPass — Design Specification

**Version:** 1.0
**Date:** 2026-04-05
**Status:** Draft
**Author:** GarudaPass Architecture Team

---

## 1. Overview

### 1.1 What Is GarudaPass

GarudaPass is Indonesia's first unified private-sector identity infrastructure platform. It combines identity verification, corporate authorization, digital signing, and verified data sharing into a single developer-friendly API — adopting proven patterns from Singapore's SingPass/CorpPass, India's Aadhaar/MCA21, UK's Companies House, and Estonia's X-Road.

### 1.2 Positioning

- **Private-sector identity layer** — not a government platform, not competing with INApas
- **Developer-first** — Stripe/Clerk-level DX with transparent pricing, instant sandbox, modern SDKs
- **Intermediary over government data** — integrates DJK (Dukcapil), BKPM (OSS/NIB), and Kemenkumham (AHU/SABH) as authoritative data sources
- **Compliant** — UU PDP, UU ITE, PP 71/2019, OJK regulations

### 1.3 Strategic Moat

The founding team is a Kemenkumham software vendor with direct access to:
- **DJK (Dukcapil)** — 200M+ biometric records, NIK verification
- **BKPM (OSS)** — NIB business licensing, entity verification
- **Kemenkumham AHU/SABH** — legal entity data with NIK of every director, commissioner, and shareholder

This triple data source creates a person-to-company identity graph that no competitor can replicate without equivalent government relationships.

---

## 2. Components

### 2.1 GarudaPass Login (Identity Provider)

OIDC Provider built on Keycloak (FAPI 2.0 certified).

**Capabilities:**
- FAPI 2.0 Security Profile with PAR (RFC 9126), DPoP (RFC 9449), PKCE S256 (RFC 7636)
- CIBA for step-up authentication via push notification
- NIK-based registration with Dukcapil demographic + biometric verification
- Multi-factor authentication: password/PIN + device biometric (FIDO2/passkeys) + face verification
- Step-up authentication engine: risk score (0-100) based on device, location, behavior, network signals
- QR code cross-device authentication (phone authenticates desktop session)

**Auth levels:**
| Level | Method | Use Case |
|-------|--------|----------|
| L0 | Session cookie / remember-me | Low-risk browsing |
| L1 | Password/PIN | Standard access |
| L2 | L1 + OTP / push / passkey | Transactions |
| L3 | L2 + face verification vs Dukcapil | High-value transactions |
| L4 | L3 + PKI signing (device key) | Legal document signing |

**Token structure:**
- ID Token: JWS-in-JWE (signed then encrypted) for tokens containing PII
- Access Token: DPoP-bound, 10-minute lifetime
- Client authentication: private_key_jwt (ES256) only — no shared secrets

### 2.2 GarudaInfo (Verified Personal Data API)

Consent-based verified data sharing, modeled after Singapore's MyInfo.

**Data categories (from Dukcapil + partner sources):**
- Identity: NIK (masked), name, DOB, gender, nationality, marital status
- Contact: phone, email, registered address
- Documents: e-KTP status, passport (via Imigrasi if available)

**Consent model:**
- Per-field, per-purpose, per-duration consent
- User sees exactly which fields are requested and by whom
- Consent is withdrawable; withdrawal triggers cascading data deletion at the requesting party
- Consent receipts provided to users (Kantara Initiative spec)
- All consent events logged immutably to Kafka audit trail

**API flow (OAuth 2.0 + PKCE):**
1. Developer's app redirects user to GarudaPass consent screen
2. User authenticates (step-up if needed) and reviews requested data fields
3. User grants consent → GarudaPass issues authorization code
4. Developer exchanges code for access token (via BFF, never browser)
5. Developer calls GarudaInfo person API with access token → receives verified data
6. Data includes `source` and `lastVerified` metadata per field

### 2.3 GarudaCorp (Corporate Identity & Authorization)

Corporate identity layer modeled after Singapore's CorpPass, powered by Kemenkumham AHU data.

**Entity verification (triple cross-reference):**
1. **Kemenkumham AHU** — SK Pengesahan, legal entity status, directors (NIK), shareholders (NIK), UBO
2. **BKPM OSS** — NIB, business activity (KBLI), operating location
3. **Dukcapil** — verify each officer's NIK against population database

**Role hierarchy (inspired by CorpPass):**
| Role | Authority Source | Capabilities |
|------|-----------------|-------------|
| Direktur Utama (Registered Officer) | AHU Akta Pendirian | Full authority. Appoints Admins. |
| Direktur (Admin) | AHU Akta + appointment | Manages users, assigns service access |
| Kuasa/Staff (User) | Delegation by Admin | Limited to assigned transaction types |
| Third-Party (Auditor/Lawyer) | Surat Kuasa | Scoped access on behalf of entity |

**Token structure (FAPI 2.0):**
- Primary subject (`sub`) = entity (SK Kemenkumham number as primary identifier; NIB as secondary cross-reference)
- Acting user (`act` claim, RFC 8693) = individual (NIK-linked UUID)
- `authinfo` scope returns role, appointment date, access rights array

**Corporate data API (GarudaCorp Info):**
- Company profile: name, status, type, capital structure, registered address
- Officers: directors + commissioners with roles and appointment dates
- Shareholders: names, share types, percentages
- Beneficial ownership: UBO data per PP 13/2018
- Fidusia: active fiduciary registrations (collateral, creditors, values)

### 2.4 GarudaSign (Digital Signing)

PKI-based digital signing using EJBCA (CA) and EU DSS (signing engine).

**Architecture:**
- **EJBCA** — Certificate Authority: 3-tier hierarchy (Root CA offline → Subordinate CA → Issuing CA), HSM-backed (FIPS 140-2 Level 3)
- **EU DSS** — Signing engine: PAdES B-LTA (PDF), XAdES (XML), CAdES (binary), JAdES (JSON)
- **RFC 3161 TSA** — Timestamping via SignServer or EJBCA TSA
- **BSrE integration** — optional pathway for government-grade certificates

**Signing flow:**
1. Document upload → hash computation
2. Signer selection (individual or corporate officer)
3. Step-up authentication to L4 (biometric + device key)
4. Private key signs document hash inside Secure Enclave (mobile) or cloud HSM (web)
5. EU DSS constructs PAdES-B-LTA: embed signature + timestamp + cert chain + OCSP responses
6. Signed document delivered to all parties

**TTE levels supported:**
| Level | Identity Verification | Key Storage | Legal Weight |
|-------|----------------------|-------------|-------------|
| Level 3 | PSrE-verified (biometric + eKTP) | Server-side HSM | High — non-repudiation |
| Level 4 | Level 3 + QSCD | Device Secure Enclave | Highest — equivalent to wet signature |

**Multi-party signing:** Sequential and parallel signing with configurable approval chains. Witness/notary participation supported.

---

## 3. Architecture

### 3.1 System Layers

```
PRESENTATION:  Citizen Portal | Admin Portal | Dev Portal (Next.js SSR) | Mobile App (Flutter)
                    │                │              │                          │
BFF LAYER:     Web BFF (Go) — sessions in Redis, token storage         │ (no BFF)
                    │                                                        │
API GATEWAY:   Kong — rate limiting, WAF, bot protection, DPoP validation, mTLS
                    │
SERVICE LAYER: Keycloak (OIDC) | GarudaInfo (Go) | GarudaCorp (Go) | GarudaSign (Go+EJBCA+DSS)
                    │
EVENT BUS:     Apache Kafka — immutable audit logs, lifecycle events, consent events
                    │
DATA LAYER:    PostgreSQL (primary, pgcrypto, RLS) | ScyllaDB (sessions/tokens) | Redis (cache)
                    │
SECURITY:      HashiCorp Vault + HSM (CloudHSM or on-prem) — signing keys, encryption keys, secrets
                    │
EXTERNAL:      DJK/Dukcapil | BKPM/OSS | Kemenkumham AHU | BSrE (optional)
```

### 3.2 Backend-for-Frontend (BFF)

All web frontends communicate through a Go BFF. The BFF:
- Is the OIDC confidential client (registered in Keycloak with client_id + private_key_jwt)
- Handles the full authorization code flow with PKCE
- Stores access/refresh tokens server-side in Redis (never exposed to browser)
- Sets HttpOnly, Secure, SameSite=Strict session cookies
- Aggregates calls to backend services
- Implements CSRF protection (double-submit cookie)

Mobile app (Flutter) does NOT use BFF. It connects directly via Kong using PKCE + DPoP + platform-secure token storage (iOS Keychain / Android Keystore).

### 3.3 Technology Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| Identity Provider | Keycloak 26+ | Only FAPI 2.0 certified OSS. Java SPI for custom flows. Apache 2.0. |
| Backend services | Go | Ory ecosystem precedent. SingPass uses Go. Small binaries, fast startup. Growing Indonesia talent pool (Gojek/Tokopedia). |
| PKI / CA | EJBCA Enterprise | Common Criteria certified. HSM support. 20+ year track record. eID/ePassport deployments. |
| Signing engine | EU DSS | Gold standard for AdES signatures. eIDAS compliant. PAdES/XAdES/CAdES/JAdES. LGPL. |
| Database (primary) | PostgreSQL | pgcrypto for field-level encryption. Row-level security. Keycloak native support. |
| Database (sessions) | ScyllaDB | Low-latency, high-throughput for token/session storage at scale. |
| Cache | Redis | BFF session store. Rate limiting counters. JWKS caching. |
| Event streaming | Apache Kafka | Immutable append-only audit log. Aadhaar uses Kafka (4 clusters). |
| Internal messaging | NATS | Lightweight service-to-service messaging. |
| API Gateway | Kong | Richest plugin ecosystem for identity (OIDC, rate limiting, bot detection). |
| Frontend portals | Next.js 14+ | SSR for security. React ecosystem. Largest hiring pool in Indonesia. |
| Mobile app | Flutter or KMP | Flutter: cross-platform, popular in Indonesia. KMP: native Secure Enclave access. Decision deferred to implementation phase. |
| IaC | OpenTofu | Terraform-compatible, truly open source. |
| Orchestration | Kubernetes | Industry standard. All reference platforms use it. |
| Observability | OpenTelemetry + Grafana (Prometheus/Loki/Tempo) | CNCF standard. Fully open source. |
| Secrets / HSM | HashiCorp Vault + CloudHSM | Transit engine for encryption-as-a-service. PKI engine for internal TLS. |
| Monorepo | Turborepo + Go workspaces | Shared contracts, atomic changes, consistent CI/CD. |

### 3.4 Verifiable Credentials (Future-Ready)

Built on W3C Verifiable Credentials 2.0 + OpenID4VCI/VP:
- **ACA-Py** (OpenWallet Foundation) for VC issuance/verification
- **AnonCreds v2** for ZKP selective disclosure (BBS+ signatures)
- **walt.id** for OID4VCI/OID4VP protocol support
- Positioned for ASEAN DEFA cross-border identity interoperability (target signing Nov 2026)

---

## 4. Security Architecture

### 4.1 Protocol Security (FAPI 2.0)

| Attack Vector | Mitigation |
|--------------|-----------|
| Auth code interception | PKCE S256 + PAR + one-time codes |
| Token theft | DPoP sender-constrained tokens |
| Token replay | DPoP proof (htm/htu/jti/ath binding) |
| Parameter tampering | PAR (back-channel, authenticated) |
| Redirect URI attack | Exact match + pre-registration |
| Mix-up attack | RFC 9207 iss parameter |
| Client impersonation | private_key_jwt (asymmetric, ES256) |

### 4.2 Data Security

- **Field-level encryption**: AES-256-GCM with envelope encryption (DEK wrapped by KEK in Vault)
- **NIK tokenization**: Vault Transform engine with format-preserving encryption
- **Data at rest**: PostgreSQL TDE + field-level encryption
- **Data in transit**: TLS 1.3 everywhere. mTLS between services (SPIFFE/SPIRE).
- **Audit logs**: Immutable (Kafka + S3 Object Lock), hash-chained, 5-year retention per PP 71/2019

### 4.3 Biometric Security

- **Liveness detection**: Commercial solution required (iProov, FaceTec, or BioID) — no production-grade open source exists. Must be iBeta PAD Level 2+ certified (ISO 30107-3).
- **Face verification**: Match live capture against Dukcapil biometric database
- **Template protection**: Cancelable biometrics — never store raw biometric data
- **FIDO2/Passkeys**: Device-bound passkeys for AAL2+. Attestation enforcement for high-assurance flows.

### 4.4 Infrastructure Security

- **Zero Trust**: SPIFFE/SPIRE + Istio mTLS + microsegmentation
- **Rate limiting**: Multi-key (IP + user + client), sliding window for auth, token bucket for APIs
- **DDoS**: CDN → WAF → Kong → service mesh layered defense
- **Supply chain**: Sigstore/Cosign image signing, SBOM (CycloneDX), SLSA Level 3
- **Fraud detection**: Real-time risk scoring (device + location + behavior + network)

### 4.5 UU PDP Compliance

| Requirement | Implementation |
|------------|---------------|
| Consent management | Dedicated consent microservice. Per-field, per-purpose, per-duration. Withdrawable. |
| Purpose limitation | ABAC policy engine (OPA/Cedar). Every API call includes purpose parameter. |
| Data minimization | Attribute-based queries. ZKP selective disclosure. |
| Data subject rights | Self-service portal: access, correct, delete, export (JSON/CSV). |
| Breach notification | Detection → triage → scope → notify within 3×24 hours. Automated pipeline. |
| Data residency | Indonesian data center only. Network egress controls. Art. 56 compliance. |
| Field-level encryption | Per-field DEK. Enables granular access control and selective deletion. |

---

## 5. Business Flows

### 5.1 Individual Registration

1. User downloads app or visits portal
2. Enters NIK (16 digits) → system validates format
3. Dukcapil pre-verification: confirms NIK exists, person is alive, returns basic demographics
4. Email + phone OTP verification (5-min expiry, max 3 resends/24h)
5. Password creation (12+ chars, checked against compromised password DB)
6. Identity verification: KTP photo capture → selfie with passive liveness → Dukcapil biometric face match (threshold ≥ 0.75)
7. MFA enrollment: TOTP or device biometric binding
8. Account status: PENDING → ACTIVE. Welcome notification sent.

### 5.2 Corporate Registration

1. Authorized officer (Direktur Utama) logs in with personal GarudaPass account
2. Enters company SK number or company name
3. System queries AHU/SABH: retrieves company profile, officers list
4. System verifies: officer's NIK matches logged-in user's NIK in AHU director records
5. Cross-references OSS/BKPM: matches NIB to same entity (same NPWP)
6. If match: officer is auto-assigned as Registered Officer (RO) for the entity
7. RO can then invite Admins and Users, assigning specific service access per person
8. Each invited person must authenticate with their own GarudaPass account (NIK-verified)

### 5.3 Developer Integration

1. Developer signs up at developer.garudapass.id (email + password)
2. Instant sandbox environment created with synthetic NIK test data
3. API keys generated: `gp_test_` (sandbox) / `gp_live_` (production)
4. Developer integrates using SDK (Next.js, Flutter, Go, Java, Python)
5. Tests in sandbox: full eKYC, signing, corporate auth flows with test data
6. Submits for production review (automated + manual)
7. Production API keys activated. Transparent per-transaction billing begins.

### 5.4 Consent-Based Data Sharing (GarudaInfo)

1. Third-party app redirects user to GarudaPass with requested data scopes
2. User authenticates (step-up if needed based on data sensitivity)
3. Consent screen displays: requesting app name, entity name, specific fields requested, purpose, duration
4. User grants → authorization code issued
5. Third-party exchanges code for access token (server-side only)
6. Third-party calls GarudaInfo API with token → receives verified data with source metadata
7. Consent recorded immutably. User can view/revoke via GarudaPass app at any time.

---

## 6. Developer Experience

### 6.1 Time-to-First-API-Call Target: < 5 Minutes

- Sign up → API key → curl example works → under 5 minutes
- No sales call, no MoU, no contract required for sandbox

### 6.2 SDKs

| Platform | Package |
|----------|---------|
| Next.js / React | `@garudapass/nextjs` |
| Flutter / Dart | `garudapass_flutter` |
| Go | `garudapass-go` |
| Java / Kotlin | `garudapass-java` |
| Python | `garudapass-python` |
| PHP / Laravel | `garudapass-php` |

### 6.3 Embeddable UI Components

Pre-built, themeable components (Stripe Appearance API model):
- `<GarudaPassLogin />` — drop-in login widget
- `<GarudaKYC />` — full eKYC flow (document + selfie + liveness)
- `<GarudaSign />` — document signing modal
- `<GarudaConsent />` — consent dialog for data sharing

Customizable via themes + variables + CSS-like rules. Shadow DOM for encapsulation. Figma kit provided.

### 6.4 Developer Portal

- Interactive API explorer with live requests
- Copy-paste code snippets in all supported languages
- Webhook testing interface (CLI + web)
- Sandbox/production toggle
- Usage dashboard with real-time metrics
- Transparent pricing calculator

### 6.5 Pricing Model

- **Per-transaction**: eKYC verification, digital signature, face verification, data API query
- **Transparent**: pricing published on website, no hidden fees
- **Tiers**: Starter (UMKM-friendly) → Growth → Enterprise
- **Free tier**: sandbox unlimited, production 100 free verifications/month

---

## 7. Revenue Streams

| Stream | Model | Example |
|--------|-------|---------|
| Per-transaction | Pay-as-you-go | eKYC: Rp 5,000/verification. Signature: Rp 3,000/sign. |
| Subscription | Monthly tiers | Starter: Rp 500K/mo. Growth: Rp 5M/mo. Enterprise: custom. |
| Embedded finance | Revenue share | Instant lending/micro-insurance enabled by verified identity data |
| Data API | Per-query | GarudaInfo verified data query: Rp 2,000/query (consent-based) |

---

## 8. Phasing

### Phase 1 — Foundation (Months 1-6)

- Keycloak deployment with FAPI 2.0 configuration
- Go BFF + Next.js citizen portal + admin portal
- Dukcapil integration (NIK verification + biometric face match)
- GarudaInfo MVP (basic personal data API with consent)
- AHU/SABH integration (company profile + officers)
- GarudaCorp MVP (entity verification + officer role assignment)
- EJBCA deployment (3-tier CA) + EU DSS (PAdES signing)
- GarudaSign MVP (individual document signing)
- Developer portal with sandbox
- PostgreSQL + Redis + Kafka setup
- Kubernetes deployment with basic monitoring

### Phase 2 — Scale (Months 6-12)

- OSS/BKPM integration (NIB cross-reference)
- Beneficial ownership API (bo.ahu.go.id)
- Fidusia data integration
- Corporate multi-party signing
- GarudaCorp role delegation (Admin → User)
- Third-party authorization flows
- Flutter mobile app
- Passkey/FIDO2 enrollment
- Adaptive risk engine (device + location + behavior scoring)
- ScyllaDB for session/token storage
- Advanced rate limiting and bot protection

### Phase 3 — Differentiate (Months 12-18)

- W3C Verifiable Credentials issuance
- ZKP selective disclosure (AnonCreds v2 / BBS+)
- Embeddable UI components (`<GarudaPassLogin />`, `<GarudaKYC />`)
- Immigration data integration (foreigner verification)
- DJKI/HKI integration (IP portfolio enrichment)
- Embedded finance APIs (income verification for lending)
- ASEAN DEFA cross-border identity readiness
- Multi-region deployment
- PSrE certification application

---

## 9. Success Criteria

| Metric | 6 Months | 12 Months | 18 Months |
|--------|----------|-----------|-----------|
| Registered developers | 100 | 1,000 | 5,000 |
| API integrations (production) | 10 | 100 | 500 |
| Individual verifications | 10K | 500K | 5M |
| Corporate entities verified | 500 | 5K | 50K |
| Digital signatures issued | 1K | 50K | 500K |
| Monthly recurring revenue | Rp 50M | Rp 500M | Rp 5B |
| Platform uptime | 99.9% | 99.95% | 99.99% |

---

## 10. Risks & Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|-----------|-----------|
| Dukcapil API downtime/changes | High | Medium | Local cache layer. Graceful degradation. Maintain direct relationship. |
| AHU data quality issues | Medium | Medium | Substantive verification (Oct 2025 policy) improves data quality. Cross-reference with OSS. |
| Regulatory changes (UU PDP enforcement) | Medium | High | Privacy-by-design from day one. Consent-as-a-service. Proactive compliance. |
| Competitor copies approach | Medium | Low | Data access moat (AHU + OSS + Dukcapil). 2+ year head start. Developer ecosystem lock-in. |
| Biometric spoofing / deepfakes | High | Medium | Commercial liveness detection (iBeta PAD Level 2+). Multi-signal fraud detection. |
| Key compromise | Critical | Low | HSM-backed keys. Short-lived tokens (10 min). Automated key rotation. Incident response playbook. |
| Scale beyond initial capacity | Medium | Medium | Kubernetes horizontal scaling. Database sharding plan. CDN for static assets. |

---

## Appendix A: Reference Standards

| Standard | Application |
|----------|------------|
| FAPI 2.0 (OpenID Foundation) | OIDC security profile |
| RFC 9126 (PAR) | Pushed authorization requests |
| RFC 9449 (DPoP) | Sender-constrained tokens |
| RFC 7636 (PKCE) | Authorization code protection |
| RFC 3161 (TSA) | Timestamping |
| ISO 30107 (PAD) | Biometric liveness detection |
| FIDO2 / WebAuthn | Phishing-resistant authentication |
| W3C Verifiable Credentials 2.0 | Privacy-preserving credentials |
| ETSI EN 319 142 (PAdES) | PDF digital signatures |
| UU PDP No. 27/2022 | Indonesian data protection |
| PP 71/2019 | Electronic systems and transactions |
| UU ITE No. 11/2008 | Electronic information and transactions |
| PP 13/2018 | Beneficial ownership |

## Appendix B: Global Reference Implementations

| Country | System | What GarudaPass Adopts |
|---------|--------|----------------------|
| Singapore | SingPass + CorpPass + MyInfo | FAPI 2.0, consent-based data sharing, corporate role hierarchy |
| India | Aadhaar + MCA21 + DIN | NIK-as-DIN person-to-company graph, biometric verification at scale |
| UK | Companies House | Open corporate data API model, PSC/UBO transparency |
| Estonia | X-Road + e-Business Registry | Peer-to-peer data exchange, cryptographic audit trail |
| Brazil | GOV.BR | Scale reference (170M users), facial biometric verification |
