CREATE TABLE IF NOT EXISTS entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ahu_sk_number VARCHAR(100) UNIQUE NOT NULL,
    name VARCHAR(500) NOT NULL,
    entity_type VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    npwp VARCHAR(20),
    address TEXT,
    capital_authorized BIGINT,
    capital_paid BIGINT,
    ahu_verified_at TIMESTAMPTZ,
    oss_nib VARCHAR(20),
    oss_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_entities_sk ON entities(ahu_sk_number);
CREATE INDEX IF NOT EXISTS idx_entities_npwp ON entities(npwp);
