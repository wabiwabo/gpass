CREATE TABLE IF NOT EXISTS signing_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    certificate_id UUID REFERENCES signing_certificates(id),
    document_name VARCHAR(255) NOT NULL,
    document_size BIGINT NOT NULL,
    document_hash VARCHAR(64) NOT NULL,
    document_path VARCHAR(500) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    error_message TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_signing_requests_user_id ON signing_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_signing_requests_status ON signing_requests(status);
CREATE INDEX IF NOT EXISTS idx_signing_requests_expires_at ON signing_requests(expires_at) WHERE status = 'PENDING';
