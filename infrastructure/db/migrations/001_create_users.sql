CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    keycloak_id UUID UNIQUE NOT NULL,
    nik_token VARCHAR(64) UNIQUE NOT NULL,
    nik_masked VARCHAR(20) NOT NULL,
    name_enc BYTEA NOT NULL,
    dob_enc BYTEA NOT NULL,
    gender VARCHAR(1) NOT NULL,
    phone_hash VARCHAR(64) NOT NULL,
    phone_enc BYTEA NOT NULL,
    email_hash VARCHAR(64) NOT NULL,
    email_enc BYTEA NOT NULL,
    address_enc BYTEA,
    wrapped_dek BYTEA NOT NULL,
    auth_level SMALLINT NOT NULL DEFAULT 0,
    verification_status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    dukcapil_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_nik_token ON users(nik_token);
CREATE INDEX IF NOT EXISTS idx_users_phone_hash ON users(phone_hash);
CREATE INDEX IF NOT EXISTS idx_users_email_hash ON users(email_hash);
CREATE INDEX IF NOT EXISTS idx_users_keycloak_id ON users(keycloak_id);
