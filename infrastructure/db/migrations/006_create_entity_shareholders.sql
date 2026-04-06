CREATE TABLE IF NOT EXISTS entity_shareholders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id UUID NOT NULL REFERENCES entities(id),
    name VARCHAR(255) NOT NULL,
    share_type VARCHAR(50),
    shares BIGINT,
    percentage DECIMAL(5,2),
    source VARCHAR(20) NOT NULL DEFAULT 'AHU'
);

CREATE INDEX IF NOT EXISTS idx_shareholders_entity ON entity_shareholders(entity_id);
