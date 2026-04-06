CREATE TABLE IF NOT EXISTS signing_certificates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    serial_number VARCHAR(64) UNIQUE NOT NULL,
    issuer_dn VARCHAR(500) NOT NULL,
    subject_dn VARCHAR(500) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    valid_from TIMESTAMPTZ NOT NULL,
    valid_to TIMESTAMPTZ NOT NULL,
    certificate_pem TEXT NOT NULL,
    fingerprint_sha256 VARCHAR(64) NOT NULL,
    revoked_at TIMESTAMPTZ,
    revocation_reason VARCHAR(50),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_signing_certs_user_id ON signing_certificates(user_id);
CREATE INDEX IF NOT EXISTS idx_signing_certs_status ON signing_certificates(status);
CREATE INDEX IF NOT EXISTS idx_signing_certs_serial ON signing_certificates(serial_number);
CREATE INDEX IF NOT EXISTS idx_signing_certs_fingerprint ON signing_certificates(fingerprint_sha256);
CREATE INDEX IF NOT EXISTS idx_signing_certs_valid_to ON signing_certificates(valid_to) WHERE status = 'ACTIVE';
