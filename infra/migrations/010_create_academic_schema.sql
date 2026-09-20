-- ==============================================================================
-- Migration: 010_create_academic_schema.sql
-- Description: Academic Context Graph, Timetable, Knowledge State, and Proactive Interventions
-- ==============================================================================

CREATE SCHEMA IF NOT EXISTS academic;

-- 1. Institutions
CREATE TABLE IF NOT EXISTS academic.institutions (
    id VARCHAR PRIMARY KEY,
    name VARCHAR NOT NULL,
    code VARCHAR NOT NULL,
    domain VARCHAR,
    settings JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- 2. Faculties
CREATE TABLE IF NOT EXISTS academic.faculties (
    id VARCHAR PRIMARY KEY,
    institution_id VARCHAR NOT NULL REFERENCES academic.institutions(id) ON DELETE CASCADE,
    name VARCHAR NOT NULL,
    code VARCHAR NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_academic_faculties_inst ON academic.faculties(institution_id);

-- 3. Departments
CREATE TABLE IF NOT EXISTS academic.departments (
    id VARCHAR PRIMARY KEY,
    faculty_id VARCHAR NOT NULL REFERENCES academic.faculties(id) ON DELETE CASCADE,
    institution_id VARCHAR NOT NULL,
    name VARCHAR NOT NULL,
    code VARCHAR NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_academic_depts_faculty ON academic.departments(faculty_id);
CREATE INDEX IF NOT EXISTS idx_academic_depts_inst ON academic.departments(institution_id);

-- 4. Courses
CREATE TABLE IF NOT EXISTS academic.courses (
    id VARCHAR PRIMARY KEY,
    institution_id VARCHAR NOT NULL REFERENCES academic.institutions(id) ON DELETE CASCADE,
    department_id VARCHAR NOT NULL REFERENCES academic.departments(id) ON DELETE CASCADE,
    code VARCHAR NOT NULL,
    title VARCHAR NOT NULL,
    level INTEGER DEFAULT 300,
    credit_units INTEGER DEFAULT 3,
    syllabus JSONB,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_academic_courses_dept ON academic.courses(department_id);
CREATE INDEX IF NOT EXISTS idx_academic_courses_code ON academic.courses(institution_id, code);
CREATE INDEX IF NOT EXISTS idx_academic_courses_inst ON academic.courses(institution_id);

-- 5. Course Schedules (Timetable)
CREATE TABLE IF NOT EXISTS academic.course_schedules (
    id VARCHAR PRIMARY KEY,
    course_id VARCHAR NOT NULL REFERENCES academic.courses(id) ON DELETE CASCADE,
    institution_id VARCHAR NOT NULL,
    day_of_week INTEGER NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    venue VARCHAR,
    recurrence VARCHAR DEFAULT 'weekly'
);
CREATE INDEX IF NOT EXISTS idx_academic_schedules_course ON academic.course_schedules(course_id);
CREATE INDEX IF NOT EXISTS idx_academic_schedules_day ON academic.course_schedules(day_of_week);
CREATE INDEX IF NOT EXISTS idx_academic_schedules_inst ON academic.course_schedules(institution_id);

-- 6. Student Enrollments
CREATE TABLE IF NOT EXISTS academic.student_enrollments (
    id VARCHAR PRIMARY KEY,
    user_id VARCHAR NOT NULL,
    course_id VARCHAR NOT NULL REFERENCES academic.courses(id) ON DELETE CASCADE,
    semester VARCHAR DEFAULT '2025/2026_FIRST',
    status VARCHAR DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_academic_enrollments_user ON academic.student_enrollments(user_id);
CREATE INDEX IF NOT EXISTS idx_academic_enrollments_course ON academic.student_enrollments(course_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_academic_enrollments_user_course ON academic.student_enrollments(user_id, course_id);

-- 7. Lectures
CREATE TABLE IF NOT EXISTS academic.lectures (
    id VARCHAR PRIMARY KEY,
    course_id VARCHAR NOT NULL REFERENCES academic.courses(id) ON DELETE CASCADE,
    lecture_number INTEGER NOT NULL,
    title VARCHAR NOT NULL,
    date DATE,
    start_time TIME,
    end_time TIME,
    topics_covered JSONB,
    slides_url VARCHAR,
    transcript_doc_id VARCHAR,
    summary_text TEXT,
    is_processed BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_academic_lectures_course ON academic.lectures(course_id);
CREATE INDEX IF NOT EXISTS idx_academic_lectures_date ON academic.lectures(course_id, date);

-- 8. Academic Events (Deadlines / Exams)
CREATE TABLE IF NOT EXISTS academic.academic_events (
    id VARCHAR PRIMARY KEY,
    course_id VARCHAR NOT NULL REFERENCES academic.courses(id) ON DELETE CASCADE,
    event_type VARCHAR NOT NULL,
    title VARCHAR NOT NULL,
    description TEXT,
    due_date TIMESTAMP WITH TIME ZONE NOT NULL,
    weight_percent INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_academic_events_course ON academic.academic_events(course_id);
CREATE INDEX IF NOT EXISTS idx_academic_events_due ON academic.academic_events(due_date);

-- 9. Context Gaps
CREATE TABLE IF NOT EXISTS academic.context_gaps (
    id VARCHAR PRIMARY KEY,
    user_id VARCHAR NOT NULL,
    course_id VARCHAR NOT NULL,
    gap_type VARCHAR NOT NULL,
    prompt_question TEXT NOT NULL,
    field_target VARCHAR NOT NULL,
    is_resolved BOOLEAN DEFAULT FALSE,
    resolved_at TIMESTAMP WITH TIME ZONE,
    response_value VARCHAR,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_academic_gaps_user ON academic.context_gaps(user_id);
CREATE INDEX IF NOT EXISTS idx_academic_gaps_status ON academic.context_gaps(user_id, is_resolved);

-- 10. Student Knowledge States
CREATE TABLE IF NOT EXISTS academic.student_knowledge_states (
    id VARCHAR PRIMARY KEY,
    user_id VARCHAR NOT NULL,
    course_id VARCHAR NOT NULL REFERENCES academic.courses(id) ON DELETE CASCADE,
    topic VARCHAR NOT NULL,
    subtopic VARCHAR NOT NULL,
    mastery_score INTEGER DEFAULT 0,
    status VARCHAR DEFAULT 'not_started',
    total_attempts INTEGER DEFAULT 0,
    correct_attempts INTEGER DEFAULT 0,
    last_evaluated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_error_summary TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_academic_ks_user_course ON academic.student_knowledge_states(user_id, course_id);
CREATE INDEX IF NOT EXISTS idx_academic_ks_status ON academic.student_knowledge_states(user_id, status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_academic_ks_unique ON academic.student_knowledge_states(user_id, course_id, topic, subtopic);

-- 11. Learning Signals
CREATE TABLE IF NOT EXISTS academic.learning_signals (
    id VARCHAR PRIMARY KEY,
    user_id VARCHAR NOT NULL,
    course_id VARCHAR NOT NULL REFERENCES academic.courses(id) ON DELETE CASCADE,
    topic VARCHAR NOT NULL,
    subtopic VARCHAR NOT NULL,
    signal_type VARCHAR NOT NULL,
    score INTEGER NOT NULL,
    max_score INTEGER DEFAULT 1,
    raw_details JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_academic_signals_user_course ON academic.learning_signals(user_id, course_id);
CREATE INDEX IF NOT EXISTS idx_academic_signals_created ON academic.learning_signals(created_at);

-- 12. Learning Gaps
CREATE TABLE IF NOT EXISTS academic.learning_gaps (
    id VARCHAR PRIMARY KEY,
    user_id VARCHAR NOT NULL,
    course_id VARCHAR NOT NULL REFERENCES academic.courses(id) ON DELETE CASCADE,
    topic VARCHAR NOT NULL,
    subtopic VARCHAR NOT NULL,
    gap_description TEXT NOT NULL,
    severity VARCHAR DEFAULT 'medium',
    status VARCHAR DEFAULT 'active',
    resolved_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_academic_gaps_student_course ON academic.learning_gaps(user_id, course_id);

-- 13. Lecturer Class Signals
CREATE TABLE IF NOT EXISTS academic.lecturer_class_signals (
    id VARCHAR PRIMARY KEY,
    course_id VARCHAR NOT NULL REFERENCES academic.courses(id) ON DELETE CASCADE,
    topic VARCHAR NOT NULL,
    subtopic VARCHAR NOT NULL,
    cohort VARCHAR DEFAULT '2025/2026_FIRST',
    total_students INTEGER DEFAULT 0,
    struggling_count INTEGER DEFAULT 0,
    struggling_percentage INTEGER DEFAULT 0,
    recommendation TEXT,
    generated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_academic_class_signals_course ON academic.lecturer_class_signals(course_id);

-- 14. Proactive Interventions (Governor & Timeline Records)
CREATE TABLE IF NOT EXISTS academic.proactive_interventions (
    id VARCHAR PRIMARY KEY,
    user_id VARCHAR NOT NULL,
    course_id VARCHAR REFERENCES academic.courses(id) ON DELETE CASCADE,
    event_type VARCHAR NOT NULL,
    card_type VARCHAR NOT NULL,
    title VARCHAR NOT NULL,
    payload JSONB,
    is_dismissed BOOLEAN DEFAULT FALSE,
    is_acted_upon BOOLEAN DEFAULT FALSE,
    delivered_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_academic_interventions_user ON academic.proactive_interventions(user_id);
CREATE INDEX IF NOT EXISTS idx_academic_interventions_delivered ON academic.proactive_interventions(delivered_at);
CREATE INDEX IF NOT EXISTS idx_academic_interventions_user_delivered ON academic.proactive_interventions(user_id, delivered_at);

-- 15. Partner Conversations (Course-Scoped Partner Memory)
CREATE TABLE IF NOT EXISTS academic.partner_conversations (
    id VARCHAR PRIMARY KEY,
    user_id VARCHAR NOT NULL,
    course_id VARCHAR NOT NULL REFERENCES academic.courses(id) ON DELETE CASCADE,
    session_id VARCHAR NOT NULL,
    role VARCHAR NOT NULL,
    content TEXT NOT NULL,
    widget_type VARCHAR,
    widget_data JSONB,
    provenance_citations JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_academic_partner_conv_user_course ON academic.partner_conversations(user_id, course_id);
CREATE INDEX IF NOT EXISTS idx_academic_partner_conv_session ON academic.partner_conversations(session_id, created_at);
