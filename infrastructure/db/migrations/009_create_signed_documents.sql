CREATE TABLE IF NOT EXISTS signed_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID UNIQUE NOT NULL REFERENCES signing_requests(id),
    certificate_id UUID NOT NULL REFERENCES signing_certificates(id),
    signed_hash VARCHAR(64) NOT NULL,
    signed_path VARCHAR(500) NOT NULL,
    signed_size BIGINT NOT NULL,
    pades_level VARCHAR(20) NOT NULL,
    signature_timestamp TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_signed_docs_request_id ON signed_documents(request_id);
CREATE INDEX IF NOT EXISTS idx_signed_docs_certificate_id ON signed_documents(certificate_id);
