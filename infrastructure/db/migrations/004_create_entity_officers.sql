CREATE TABLE IF NOT EXISTS entity_officers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id UUID NOT NULL REFERENCES entities(id),
    user_id UUID,
    nik_token VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    position VARCHAR(50) NOT NULL,
    appointment_date DATE,
    source VARCHAR(20) NOT NULL DEFAULT 'AHU',
    verified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_officers_entity ON entity_officers(entity_id);
CREATE INDEX IF NOT EXISTS idx_officers_nik ON entity_officers(nik_token);
CREATE INDEX IF NOT EXISTS idx_officers_user ON entity_officers(user_id) WHERE user_id IS NOT NULL;
