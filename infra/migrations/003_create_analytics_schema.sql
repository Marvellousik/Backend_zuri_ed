-- Migration: Create analytics schema for Analytics Service
-- This migration sets up tables for quiz tracking, study sessions, and learning analytics
-- Unified canonical schema: zuri_analytics with backward-compatible analytics views

-- Create schemas
CREATE SCHEMA IF NOT EXISTS zuri_analytics;
CREATE SCHEMA IF NOT EXISTS analytics;

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

-- Topic mastery tracking
CREATE TABLE IF NOT EXISTS zuri_analytics.topic_mastery (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    course_id UUID NOT NULL REFERENCES zuri_content.courses(id) ON DELETE CASCADE,
    topic VARCHAR(255) NOT NULL,
    mastery_level DECIMAL(3,2) DEFAULT 0.00, -- 0.00 to 1.00 (0% to 100%)
    quizzes_taken INTEGER DEFAULT 0,
    last_practiced_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, course_id, topic)
);

-- Create indexes for topic mastery
CREATE INDEX IF NOT EXISTS idx_topic_mastery_user_id ON zuri_analytics.topic_mastery(user_id);
CREATE INDEX IF NOT EXISTS idx_topic_mastery_course_id ON zuri_analytics.topic_mastery(course_id);
CREATE INDEX IF NOT EXISTS idx_topic_mastery_mastery ON zuri_analytics.topic_mastery(mastery_level);

-- AI interaction logs
CREATE TABLE IF NOT EXISTS zuri_analytics.ai_interactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    interaction_type VARCHAR(50) NOT NULL, -- chat, generate_quiz, generate_flashcards, summarize, tts
    input_tokens INTEGER,
    output_tokens INTEGER,
    model_name VARCHAR(100),
    response_time_ms INTEGER,
    status VARCHAR(20) DEFAULT 'success', -- success, error, timeout
    error_message TEXT,
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
    title VARCHAR(255) NOT NULL,
    description TEXT,
    target_value INTEGER NOT NULL,
    current_value INTEGER DEFAULT 0,
    unit VARCHAR(50) NOT NULL, -- minutes, quizzes, flashcards, courses
    goal_type VARCHAR(50) DEFAULT 'study_time',
    period VARCHAR(20) NOT NULL, -- daily, weekly, monthly
    start_date DATE NOT NULL,
    end_date DATE,
    is_completed BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for learning goals
CREATE INDEX IF NOT EXISTS idx_learning_goals_user_id ON zuri_analytics.learning_goals(user_id);
CREATE INDEX IF NOT EXISTS idx_learning_goals_period ON zuri_analytics.learning_goals(period);
CREATE INDEX IF NOT EXISTS idx_learning_goals_completed ON zuri_analytics.learning_goals(is_completed);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION zuri_analytics.update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers to automatically update updated_at
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

-- Backward compatibility views for legacy or external callers expecting analytics.*
CREATE OR REPLACE VIEW analytics.quiz_attempts AS SELECT * FROM zuri_analytics.quiz_attempts;
CREATE OR REPLACE VIEW analytics.quiz_answers AS SELECT * FROM zuri_analytics.quiz_answers;
CREATE OR REPLACE VIEW analytics.study_sessions AS SELECT * FROM zuri_analytics.study_sessions;
CREATE OR REPLACE VIEW analytics.topic_mastery AS SELECT * FROM zuri_analytics.topic_mastery;
CREATE OR REPLACE VIEW analytics.ai_interactions AS SELECT * FROM zuri_analytics.ai_interactions;
CREATE OR REPLACE VIEW analytics.learning_goals AS SELECT * FROM zuri_analytics.learning_goals;
