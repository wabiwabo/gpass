-- infrastructure/db/migrations/011_create_api_keys.sql
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES developer_apps(id),
    key_hash VARCHAR(64) NOT NULL,                       -- SHA-256 of plaintext key
    key_prefix VARCHAR(16) NOT NULL,                     -- gp_live_ or gp_test_ + first 8 chars
    name VARCHAR(100) NOT NULL DEFAULT 'Default',
    environment VARCHAR(10) NOT NULL,                    -- sandbox, production
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',        -- ACTIVE, REVOKED
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,                              -- optional expiry
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_keys_app ON api_keys(app_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix);
CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys(status);
