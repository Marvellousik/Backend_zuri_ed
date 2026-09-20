-- Migration: supabase/015_create_administration_and_audit_zuri.sql
-- Description: Creates institutional administration, audit logging, and SIS import tables for Supabase

CREATE SCHEMA IF NOT EXISTS zuri_academic;
CREATE SCHEMA IF NOT EXISTS academic;

-- 1. Institutional Audit Logs
CREATE TABLE IF NOT EXISTS zuri_academic.audit_logs (
    id VARCHAR(64) PRIMARY KEY,
    institution_id VARCHAR(100) NOT NULL,
    actor_id VARCHAR(100) NOT NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(100) NOT NULL,
    resource_id VARCHAR(100) NOT NULL,
    payload JSONB,
    ip_address VARCHAR(45),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_inst ON zuri_academic.audit_logs(institution_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON zuri_academic.audit_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON zuri_academic.audit_logs(action);

-- 2. Student Information System (SIS) Bulk Imports
CREATE TABLE IF NOT EXISTS zuri_academic.sis_imports (
    id VARCHAR(64) PRIMARY KEY,
    institution_id VARCHAR(100) NOT NULL,
    import_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    total_records INTEGER DEFAULT 0,
    processed_records INTEGER DEFAULT 0,
    errors JSONB,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sis_imports_inst ON zuri_academic.sis_imports(institution_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_sis_imports_status ON zuri_academic.sis_imports(status);

-- 3. Backward Compatibility Views
CREATE OR REPLACE VIEW academic.audit_logs AS SELECT * FROM zuri_academic.audit_logs;
CREATE OR REPLACE VIEW academic.sis_imports AS SELECT * FROM zuri_academic.sis_imports;
