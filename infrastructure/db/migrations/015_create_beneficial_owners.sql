CREATE TABLE IF NOT EXISTS beneficial_owners (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id UUID NOT NULL REFERENCES entities(id),
    name VARCHAR(255) NOT NULL,
    nik_token VARCHAR(64) NOT NULL,
    ownership_type VARCHAR(30) NOT NULL,
    percentage DECIMAL(5,2),
    source VARCHAR(20) NOT NULL DEFAULT 'AHU',
    criteria VARCHAR(20) NOT NULL DEFAULT 'PP_13_2018',
    status VARCHAR(30) NOT NULL DEFAULT 'IDENTIFIED',
    analyzed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ubo_entity ON beneficial_owners(entity_id);
CREATE INDEX IF NOT EXISTS idx_ubo_nik ON beneficial_owners(nik_token);
CREATE INDEX IF NOT EXISTS idx_ubo_status ON beneficial_owners(status);
