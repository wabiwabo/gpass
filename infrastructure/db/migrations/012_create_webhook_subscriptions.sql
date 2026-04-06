-- infrastructure/db/migrations/012_create_webhook_subscriptions.sql
CREATE TABLE IF NOT EXISTS webhook_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES developer_apps(id),
    url VARCHAR(2048) NOT NULL,
    events TEXT[] NOT NULL,                               -- e.g. {identity.verified, document.signed}
    secret VARCHAR(64) NOT NULL,                          -- HMAC signing secret (hex)
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',         -- ACTIVE, DISABLED
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhooks_app ON webhook_subscriptions(app_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_status ON webhook_subscriptions(status);
