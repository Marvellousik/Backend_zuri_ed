-- ==============================================================================
-- Migration: supabase/012_create_programs_and_cohorts_zuri.sql
-- Description: Academic Programs, Sessions, and Semesters for Supabase / zuri_academic schema
-- ==============================================================================

CREATE SCHEMA IF NOT EXISTS zuri_academic;
CREATE SCHEMA IF NOT EXISTS academic;

-- 1. Academic Programs
CREATE TABLE IF NOT EXISTS zuri_academic.programs (
    id VARCHAR PRIMARY KEY,
    department_id VARCHAR NOT NULL REFERENCES academic.departments(id) ON DELETE CASCADE,
    institution_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    code VARCHAR NOT NULL,
    degree_type VARCHAR DEFAULT 'BSc',
    duration_years INTEGER DEFAULT 4,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_academic_programs_dept ON zuri_academic.programs(department_id);
CREATE INDEX IF NOT EXISTS idx_academic_programs_inst ON zuri_academic.programs(institution_id);

-- 2. Academic Sessions (e.g., 2025/2026)
CREATE TABLE IF NOT EXISTS zuri_academic.academic_sessions (
    id VARCHAR PRIMARY KEY,
    institution_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL, -- e.g. "2025/2026"
    start_date DATE,
    end_date DATE,
    is_current BOOLEAN DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_academic_sessions_inst ON zuri_academic.academic_sessions(institution_id);

-- 3. Semesters (e.g., FIRST, SECOND)
CREATE TABLE IF NOT EXISTS zuri_academic.semesters (
    id VARCHAR PRIMARY KEY,
    session_id VARCHAR NOT NULL REFERENCES zuri_academic.academic_sessions(id) ON DELETE CASCADE,
    institution_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL, -- "FIRST" or "SECOND"
    start_date DATE,
    end_date DATE,
    is_current BOOLEAN DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_academic_semesters_session ON zuri_academic.semesters(session_id);
CREATE INDEX IF NOT EXISTS idx_academic_semesters_inst ON zuri_academic.semesters(institution_id);

-- 4. Course Offerings (Specific active cohort/section of a course in a semester)
CREATE TABLE IF NOT EXISTS zuri_academic.course_offerings (
    id VARCHAR PRIMARY KEY,
    course_id VARCHAR NOT NULL REFERENCES academic.courses(id) ON DELETE CASCADE,
    institution_id VARCHAR NOT NULL,
    semester_id VARCHAR,
    lecturer_id VARCHAR,
    capacity INTEGER DEFAULT 150,
    status VARCHAR DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_academic_offerings_course ON zuri_academic.course_offerings(course_id);
CREATE INDEX IF NOT EXISTS idx_academic_offerings_inst ON zuri_academic.course_offerings(institution_id);
CREATE INDEX IF NOT EXISTS idx_academic_offerings_semester ON zuri_academic.course_offerings(semester_id);
CREATE INDEX IF NOT EXISTS idx_academic_offerings_lecturer ON zuri_academic.course_offerings(lecturer_id);

-- 5. Backward-Compatible Views in academic schema
CREATE OR REPLACE VIEW academic.programs AS SELECT * FROM zuri_academic.programs;
CREATE OR REPLACE VIEW academic.academic_sessions AS SELECT * FROM zuri_academic.academic_sessions;
CREATE OR REPLACE VIEW academic.semesters AS SELECT * FROM zuri_academic.semesters;
CREATE OR REPLACE VIEW academic.course_offerings AS SELECT * FROM zuri_academic.course_offerings;

-- 6. Enhance academic.student_enrollments with offering ID, enrollment type, and grade
ALTER TABLE academic.student_enrollments
    ADD COLUMN IF NOT EXISTS course_offering_id VARCHAR REFERENCES zuri_academic.course_offerings(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS enrollment_type VARCHAR DEFAULT 'credit',
    ADD COLUMN IF NOT EXISTS grade VARCHAR;

CREATE INDEX IF NOT EXISTS idx_academic_enrollments_offering ON academic.student_enrollments(course_offering_id);

-- 7. Forward Compatible View for zuri_academic.student_enrollments
CREATE OR REPLACE VIEW zuri_academic.student_enrollments AS SELECT * FROM academic.student_enrollments;

