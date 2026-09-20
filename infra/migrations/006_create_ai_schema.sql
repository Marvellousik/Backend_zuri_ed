-- Migration: Create AI schema for the Python AI monolith service
-- This schema isolates AI session data from the backend's auth/content/analytics schemas

CREATE SCHEMA IF NOT EXISTS ai;

-- Session type enum
CREATE TYPE ai.session_type AS ENUM ('notes', 'reading', 'flashcard', 'quiz');

-- Unified session table — one row per AI action
CREATE TABLE IF NOT EXISTS ai.user_sessions (
    session_id   VARCHAR        PRIMARY KEY,
    user_id      VARCHAR        NOT NULL,
    session_type ai.session_type NOT NULL,
    filename     VARCHAR,
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Writing assistant (notes)
    subject          VARCHAR,
    structured_notes TEXT,

    -- Reading assistant
    weaviate_collection VARCHAR,
    summary             TEXT,
    summary_type        VARCHAR,
    tts_audio_b64       TEXT,
    vocab_terms         JSONB,

    -- Study buddy — flashcards
    flashcards JSONB,
    num_cards  INTEGER,

    -- Study buddy — quiz
    quiz_type     VARCHAR,
    questions     JSONB,
    num_questions INTEGER
);

CREATE INDEX IF NOT EXISTS idx_ai_user_sessions_user_id ON ai.user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_ai_user_sessions_type    ON ai.user_sessions(session_type);
