-- Migration: 014_durable_content_provenance.sql
-- Description: Add durable content provenance, tracking, and audio transcription fields to materials

ALTER TABLE zuri_content.materials
    ADD COLUMN IF NOT EXISTS sha256_checksum VARCHAR(64),
    ADD COLUMN IF NOT EXISTS version INTEGER DEFAULT 1,
    ADD COLUMN IF NOT EXISTS course_offering_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS tracking_id VARCHAR(100),
    ADD COLUMN IF NOT EXISTS duration_seconds INTEGER,
    ADD COLUMN IF NOT EXISTS transcription_status VARCHAR(20) DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS transcription_text TEXT;

-- Indexes for tracking, course offerings, and sha256 checksums
CREATE INDEX IF NOT EXISTS idx_materials_tracking_id ON zuri_content.materials(tracking_id);
CREATE INDEX IF NOT EXISTS idx_materials_course_offering_id ON zuri_content.materials(course_offering_id);
CREATE INDEX IF NOT EXISTS idx_materials_sha256_checksum ON zuri_content.materials(sha256_checksum);

-- Update backward compatibility view
CREATE OR REPLACE VIEW content.materials AS SELECT * FROM zuri_content.materials;
