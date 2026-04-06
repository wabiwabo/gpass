CREATE TABLE IF NOT EXISTS entity_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id UUID NOT NULL REFERENCES entities(id),
    user_id UUID NOT NULL,
    role VARCHAR(20) NOT NULL,
    granted_by UUID,
    service_access JSONB NOT NULL DEFAULT '[]',
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_roles_entity ON entity_roles(entity_id);
CREATE INDEX IF NOT EXISTS idx_roles_user ON entity_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_roles_status ON entity_roles(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_unique_active ON entity_roles(entity_id, user_id) WHERE status = 'ACTIVE';
