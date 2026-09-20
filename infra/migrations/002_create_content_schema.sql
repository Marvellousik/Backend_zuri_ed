-- Migration: Create content schema for Content Service
-- This migration sets up tables for courses, materials, quizzes, and flashcards
-- Unified canonical schema: zuri_content with backward-compatible content views

-- Create schemas
CREATE SCHEMA IF NOT EXISTS zuri_content;
CREATE SCHEMA IF NOT EXISTS content;

-- Courses table
CREATE TABLE IF NOT EXISTS zuri_content.courses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    color VARCHAR(7) DEFAULT '#3B82F6',
    semester VARCHAR(20),
    year INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(user_id, name)
);

-- Create indexes for courses
CREATE INDEX IF NOT EXISTS idx_courses_user_id ON zuri_content.courses(user_id);
CREATE INDEX IF NOT EXISTS idx_courses_deleted_at ON zuri_content.courses(deleted_at);

-- Materials (files) table
CREATE TABLE IF NOT EXISTS zuri_content.materials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    course_id UUID REFERENCES zuri_content.courses(id) ON DELETE SET NULL,
    title VARCHAR(255) NOT NULL,
    file_url VARCHAR(500),
    file_size BIGINT,
    mime_type VARCHAR(100),
    processing_status VARCHAR(20) DEFAULT 'pending', -- pending, processing, completed, failed
    summary TEXT,
    audio_url VARCHAR(500),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes for materials
CREATE INDEX IF NOT EXISTS idx_materials_user_id ON zuri_content.materials(user_id);
CREATE INDEX IF NOT EXISTS idx_materials_course_id ON zuri_content.materials(course_id);
CREATE INDEX IF NOT EXISTS idx_materials_processing_status ON zuri_content.materials(processing_status);
CREATE INDEX IF NOT EXISTS idx_materials_deleted_at ON zuri_content.materials(deleted_at);

-- Quizzes table
CREATE TABLE IF NOT EXISTS zuri_content.quizzes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    course_id UUID REFERENCES zuri_content.courses(id) ON DELETE CASCADE,
    material_id UUID REFERENCES zuri_content.materials(id) ON DELETE SET NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    time_limit_minutes INTEGER,
    difficulty VARCHAR(20), -- easy, medium, hard
    shuffle_questions BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes for quizzes
CREATE INDEX IF NOT EXISTS idx_quizzes_user_id ON zuri_content.quizzes(user_id);
CREATE INDEX IF NOT EXISTS idx_quizzes_course_id ON zuri_content.quizzes(course_id);
CREATE INDEX IF NOT EXISTS idx_quizzes_material_id ON zuri_content.quizzes(material_id);
CREATE INDEX IF NOT EXISTS idx_quizzes_deleted_at ON zuri_content.quizzes(deleted_at);

-- Quiz questions table
CREATE TABLE IF NOT EXISTS zuri_content.quiz_questions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    quiz_id UUID NOT NULL REFERENCES zuri_content.quizzes(id) ON DELETE CASCADE,
    question_type VARCHAR(20) NOT NULL, -- multiple_choice, true_false, short_answer
    question_text TEXT NOT NULL,
    options JSONB, -- Array of options for multiple choice
    correct_answer TEXT NOT NULL,
    explanation TEXT,
    points INTEGER DEFAULT 1,
    order_index INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for quiz questions
CREATE INDEX IF NOT EXISTS idx_quiz_questions_quiz_id ON zuri_content.quiz_questions(quiz_id);
CREATE INDEX IF NOT EXISTS idx_quiz_questions_order ON zuri_content.quiz_questions(quiz_id, order_index);

-- Flashcard decks table
CREATE TABLE IF NOT EXISTS zuri_content.flashcard_decks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    course_id UUID REFERENCES zuri_content.courses(id) ON DELETE CASCADE,
    material_id UUID REFERENCES zuri_content.materials(id) ON DELETE SET NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    card_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes for flashcard decks
CREATE INDEX IF NOT EXISTS idx_flashcard_decks_user_id ON zuri_content.flashcard_decks(user_id);
CREATE INDEX IF NOT EXISTS idx_flashcard_decks_course_id ON zuri_content.flashcard_decks(course_id);
CREATE INDEX IF NOT EXISTS idx_flashcard_decks_deleted_at ON zuri_content.flashcard_decks(deleted_at);

-- Flashcards table
CREATE TABLE IF NOT EXISTS zuri_content.flashcards (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    deck_id UUID NOT NULL REFERENCES zuri_content.flashcard_decks(id) ON DELETE CASCADE,
    front TEXT NOT NULL,
    back TEXT NOT NULL,
    order_index INTEGER NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for flashcards
CREATE INDEX IF NOT EXISTS idx_flashcards_deck_id ON zuri_content.flashcards(deck_id);
CREATE INDEX IF NOT EXISTS idx_flashcards_order ON zuri_content.flashcards(deck_id, order_index);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION zuri_content.update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers to automatically update updated_at
DROP TRIGGER IF EXISTS update_courses_updated_at ON zuri_content.courses;
CREATE TRIGGER update_courses_updated_at
    BEFORE UPDATE ON zuri_content.courses
    FOR EACH ROW
    EXECUTE FUNCTION zuri_content.update_updated_at_column();

DROP TRIGGER IF EXISTS update_materials_updated_at ON zuri_content.materials;
CREATE TRIGGER update_materials_updated_at
    BEFORE UPDATE ON zuri_content.materials
    FOR EACH ROW
    EXECUTE FUNCTION zuri_content.update_updated_at_column();

DROP TRIGGER IF EXISTS update_quizzes_updated_at ON zuri_content.quizzes;
CREATE TRIGGER update_quizzes_updated_at
    BEFORE UPDATE ON zuri_content.quizzes
    FOR EACH ROW
    EXECUTE FUNCTION zuri_content.update_updated_at_column();

DROP TRIGGER IF EXISTS update_quiz_questions_updated_at ON zuri_content.quiz_questions;
CREATE TRIGGER update_quiz_questions_updated_at
    BEFORE UPDATE ON zuri_content.quiz_questions
    FOR EACH ROW
    EXECUTE FUNCTION zuri_content.update_updated_at_column();

DROP TRIGGER IF EXISTS update_flashcard_decks_updated_at ON zuri_content.flashcard_decks;
CREATE TRIGGER update_flashcard_decks_updated_at
    BEFORE UPDATE ON zuri_content.flashcard_decks
    FOR EACH ROW
    EXECUTE FUNCTION zuri_content.update_updated_at_column();

DROP TRIGGER IF EXISTS update_flashcards_updated_at ON zuri_content.flashcards;
CREATE TRIGGER update_flashcards_updated_at
    BEFORE UPDATE ON zuri_content.flashcards
    FOR EACH ROW
    EXECUTE FUNCTION zuri_content.update_updated_at_column();

-- Backward compatibility views for legacy or external callers expecting content.*
CREATE OR REPLACE VIEW content.courses AS SELECT * FROM zuri_content.courses;
CREATE OR REPLACE VIEW content.materials AS SELECT * FROM zuri_content.materials;
CREATE OR REPLACE VIEW content.quizzes AS SELECT * FROM zuri_content.quizzes;
CREATE OR REPLACE VIEW content.quiz_questions AS SELECT * FROM zuri_content.quiz_questions;
CREATE OR REPLACE VIEW content.flashcard_decks AS SELECT * FROM zuri_content.flashcard_decks;
CREATE OR REPLACE VIEW content.flashcards AS SELECT * FROM zuri_content.flashcards;
