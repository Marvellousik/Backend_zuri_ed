-- Migration: supabase/013_enrich_course_intelligence_zuri.sql
-- Description: Enrich Course entity with institutional bindings for Supabase

ALTER TABLE zuri_content.courses
    ADD COLUMN IF NOT EXISTS institution_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS code VARCHAR(50),
    ADD COLUMN IF NOT EXISTS course_offering_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS credit_units INTEGER DEFAULT 3,
    ADD COLUMN IF NOT EXISTS syllabus TEXT;

CREATE INDEX IF NOT EXISTS idx_courses_institution_id ON zuri_content.courses(institution_id);
CREATE INDEX IF NOT EXISTS idx_courses_course_offering_id ON zuri_content.courses(course_offering_id);
CREATE INDEX IF NOT EXISTS idx_courses_code ON zuri_content.courses(code);

CREATE OR REPLACE VIEW content.courses AS SELECT * FROM zuri_content.courses;
