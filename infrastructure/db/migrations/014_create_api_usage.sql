-- infrastructure/db/migrations/014_create_api_usage.sql
CREATE TABLE IF NOT EXISTS api_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES developer_apps(id),
    date DATE NOT NULL,
    endpoint VARCHAR(200) NOT NULL,
    call_count BIGINT NOT NULL DEFAULT 0,
    error_count BIGINT NOT NULL DEFAULT 0,
    UNIQUE(app_id, date, endpoint)
);

CREATE INDEX IF NOT EXISTS idx_usage_app_date ON api_usage(app_id, date);
