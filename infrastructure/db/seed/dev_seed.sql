-- GarudaPass Development Seed Data
-- Run after migrations: psql $DATABASE_URL -f infrastructure/db/seed/dev_seed.sql

-- Development user (matches dukcapil-sim test data)
INSERT INTO users (id, nik_token, encrypted_name, verification_status, auth_level, created_at, updated_at)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'dev_nik_token_budi_santoso',
    'encrypted_budi_santoso',
    'VERIFIED',
    2,
    NOW(),
    NOW()
) ON CONFLICT DO NOTHING;

-- Development entity (matches ahu-sim test data)
INSERT INTO entities (id, ahu_sk_number, name, entity_type, status, npwp, created_at, updated_at)
VALUES (
    'b0000000-0000-0000-0000-000000000001',
    'AHU-0012345.AH.01.01.TAHUN2024',
    'PT Maju Bersama Indonesia',
    'PT',
    'ACTIVE',
    '01.234.567.8-012.000',
    NOW(),
    NOW()
) ON CONFLICT DO NOTHING;

-- Development officer
INSERT INTO entity_officers (entity_id, user_id, nik_token, name, position, source, verified, created_at)
VALUES (
    'b0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000001',
    'dev_nik_token_budi_santoso',
    'Budi Santoso',
    'DIREKTUR_UTAMA',
    'AHU',
    true,
    NOW()
) ON CONFLICT DO NOTHING;

-- Development role
INSERT INTO entity_roles (entity_id, user_id, role, status, granted_at, created_at)
VALUES (
    'b0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000001',
    'REGISTERED_OFFICER',
    'ACTIVE',
    NOW(),
    NOW()
) ON CONFLICT DO NOTHING;

-- Development app (for Portal testing)
INSERT INTO developer_apps (id, owner_user_id, name, description, environment, tier, daily_limit, status, created_at, updated_at)
VALUES (
    'c0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000001',
    'GarudaPass Dev App',
    'Development test application',
    'sandbox',
    'free',
    100,
    'ACTIVE',
    NOW(),
    NOW()
) ON CONFLICT DO NOTHING;

-- Sample consent
INSERT INTO consents (id, user_id, requester_app_id, purpose, fields, status, granted_at, expires_at, created_at)
VALUES (
    'd0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000001',
    'c0000000-0000-0000-0000-000000000001',
    'kyc_verification',
    '{"name": true, "dob": true, "address": false}'::jsonb,
    'ACTIVE',
    NOW(),
    NOW() + INTERVAL '1 year',
    NOW()
) ON CONFLICT DO NOTHING;

-- Audit trail for seed data
INSERT INTO audit_events (event_type, actor_id, actor_type, resource_id, resource_type, action, service_name, status, metadata, created_at)
VALUES
    ('system.seed', 'SYSTEM', 'SYSTEM', 'dev_seed', 'SYSTEM', 'CREATE', 'seed', 'SUCCESS', '{"note": "development seed data"}'::jsonb, NOW());
