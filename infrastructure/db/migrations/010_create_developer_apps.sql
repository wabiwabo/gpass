-- infrastructure/db/migrations/010_create_developer_apps.sql
CREATE TABLE IF NOT EXISTS developer_apps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    environment VARCHAR(10) NOT NULL DEFAULT 'sandbox', -- sandbox, production
    tier VARCHAR(20) NOT NULL DEFAULT 'free',            -- free, starter, growth, enterprise
    daily_limit INT NOT NULL DEFAULT 100,
    callback_urls TEXT[] NOT NULL DEFAULT '{}',
    oauth_client_id VARCHAR(100),                        -- Keycloak client ID
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',        -- ACTIVE, SUSPENDED, DELETED
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dev_apps_owner ON developer_apps(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_dev_apps_status ON developer_apps(status);
CREATE INDEX IF NOT EXISTS idx_dev_apps_oauth_client ON developer_apps(oauth_client_id);
