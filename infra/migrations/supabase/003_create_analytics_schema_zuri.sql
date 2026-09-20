-- Migration: Create analytics schema for Analytics Service
-- This migration sets up tables for quiz tracking, study sessions, and learning analytics

-- Create analytics schema
CREATE SCHEMA IF NOT EXISTS zuri_analytics;

-- Quiz attempts and results
CREATE TABLE IF NOT EXISTS zuri_analytics.quiz_attempts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    quiz_id UUID NOT NULL REFERENCES zuri_content.quizzes(id) ON DELETE CASCADE,
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    score INTEGER,
    max_score INTEGER,
    percentage DECIMAL(5,2),
    time_taken_seconds INTEGER,
    status VARCHAR(20) DEFAULT 'in_progress', -- in_progress, completed, abandoned
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for quiz attempts
CREATE INDEX IF NOT EXISTS idx_quiz_attempts_user_id ON zuri_analytics.quiz_attempts(user_id);
CREATE INDEX IF NOT EXISTS idx_quiz_attempts_quiz_id ON zuri_analytics.quiz_attempts(quiz_id);
CREATE INDEX IF NOT EXISTS idx_quiz_attempts_status ON zuri_analytics.quiz_attempts(status);
CREATE INDEX IF NOT EXISTS idx_quiz_attempts_created_at ON zuri_analytics.quiz_attempts(created_at);

-- Quiz answers
CREATE TABLE IF NOT EXISTS zuri_analytics.quiz_answers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    attempt_id UUID NOT NULL REFERENCES zuri_analytics.quiz_attempts(id) ON DELETE CASCADE,
    question_id UUID NOT NULL REFERENCES zuri_content.quiz_questions(id) ON DELETE CASCADE,
    user_answer TEXT NOT NULL,
    is_correct BOOLEAN,
    similarity_score DECIMAL(5,4), -- For fuzzy matching short answers (0-1)
    points_earned INTEGER DEFAULT 0,
    time_taken_seconds INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for quiz answers
CREATE INDEX IF NOT EXISTS idx_quiz_answers_attempt_id ON zuri_analytics.quiz_answers(attempt_id);
CREATE INDEX IF NOT EXISTS idx_quiz_answers_question_id ON zuri_analytics.quiz_answers(question_id);

-- Study sessions for streak tracking
CREATE TABLE IF NOT EXISTS zuri_analytics.study_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    session_date DATE NOT NULL,
    duration_minutes INTEGER DEFAULT 0,
    materials_reviewed INTEGER DEFAULT 0,
    quizzes_completed INTEGER DEFAULT 0,
    flashcards_reviewed INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, session_date)
);

-- Create indexes for study sessions
CREATE INDEX IF NOT EXISTS idx_study_sessions_user_id ON zuri_analytics.study_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_study_sessions_session_date ON zuri_analytics.study_sessions(session_date);
CREATE INDEX IF NOT EXISTS idx_study_sessions_user_date ON zuri_analytics.study_sessions(user_id, session_date);

-- Topic mastery tracking
CREATE TABLE IF NOT EXISTS zuri_analytics.topic_mastery (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    course_id UUID REFERENCES zuri_content.courses(id) ON DELETE CASCADE,
    topic VARCHAR(255) NOT NULL,
    mastery_score DECIMAL(5,2) DEFAULT 0.00, -- 0-100
    questions_attempted INTEGER DEFAULT 0,
    questions_correct INTEGER DEFAULT 0,
    current_interval_days INTEGER DEFAULT 0, -- For spaced repetition
    last_studied_at TIMESTAMP WITH TIME ZONE,
    next_review_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, course_id, topic)
);

-- Create indexes for topic mastery
CREATE INDEX IF NOT EXISTS idx_topic_mastery_user_id ON zuri_analytics.topic_mastery(user_id);
CREATE INDEX IF NOT EXISTS idx_topic_mastery_course_id ON zuri_analytics.topic_mastery(course_id);
CREATE INDEX IF NOT EXISTS idx_topic_mastery_next_review ON zuri_analytics.topic_mastery(user_id, next_review_at);

-- AI usage tracking
CREATE TABLE IF NOT EXISTS zuri_analytics.ai_interactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    interaction_type VARCHAR(50) NOT NULL, -- quiz_generation, summary, flashcard, chat
    material_id UUID REFERENCES zuri_content.materials(id) ON DELETE SET NULL,
    prompt_tokens INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    latency_ms INTEGER,
    success BOOLEAN DEFAULT true,
    error_message TEXT,
    model VARCHAR(50), -- gpt-4, claude, gemini, etc.
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for AI interactions
CREATE INDEX IF NOT EXISTS idx_ai_interactions_user_id ON zuri_analytics.ai_interactions(user_id);
CREATE INDEX IF NOT EXISTS idx_ai_interactions_type ON zuri_analytics.ai_interactions(interaction_type);
CREATE INDEX IF NOT EXISTS idx_ai_interactions_created_at ON zuri_analytics.ai_interactions(created_at);

-- Learning goals
CREATE TABLE IF NOT EXISTS zuri_analytics.learning_goals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    course_id UUID REFERENCES zuri_content.courses(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    target_date DATE,
    target_score INTEGER, -- e.g., target quiz percentage
    is_completed BOOLEAN DEFAULT false,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for learning goals
CREATE INDEX IF NOT EXISTS idx_learning_goals_user_id ON zuri_analytics.learning_goals(user_id);
CREATE INDEX IF NOT EXISTS idx_learning_goals_course_id ON zuri_analytics.learning_goals(course_id);
CREATE INDEX IF NOT EXISTS idx_learning_goals_completed ON zuri_analytics.learning_goals(user_id, is_completed);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION zuri_analytics.update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for automatically updating updated_at
DROP TRIGGER IF EXISTS update_topic_mastery_updated_at ON zuri_analytics.topic_mastery;
CREATE TRIGGER update_topic_mastery_updated_at
    BEFORE UPDATE ON zuri_analytics.topic_mastery
    FOR EACH ROW
    EXECUTE FUNCTION zuri_analytics.update_updated_at_column();

DROP TRIGGER IF EXISTS update_learning_goals_updated_at ON zuri_analytics.learning_goals;
CREATE TRIGGER update_learning_goals_updated_at
    BEFORE UPDATE ON zuri_analytics.learning_goals
    FOR EACH ROW
    EXECUTE FUNCTION zuri_analytics.update_updated_at_column();

-- View for user study statistics
CREATE OR REPLACE VIEW zuri_analytics.user_study_stats AS
SELECT 
    user_id,
    COUNT(DISTINCT session_date) as total_study_days,
    SUM(duration_minutes) as total_study_minutes,
    SUM(quizzes_completed) as total_quizzes_completed,
    SUM(materials_reviewed) as total_materials_reviewed,
    MAX(session_date) as last_study_date
FROM zuri_analytics.study_sessions
GROUP BY user_id;

-- View for quiz performance summary
CREATE OR REPLACE VIEW zuri_analytics.quiz_performance_summary AS
SELECT 
    qa.user_id,
    qa.quiz_id,
    COUNT(*) as attempt_count,
    AVG(qa.percentage) as avg_percentage,
    MAX(qa.percentage) as best_percentage,
    MIN(qa.time_taken_seconds) as best_time_seconds
FROM zuri_analytics.quiz_attempts qa
WHERE qa.status = 'completed'
GROUP BY qa.user_id, qa.quiz_id;
