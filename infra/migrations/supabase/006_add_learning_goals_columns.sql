-- Migration: Add missing columns to learning_goals table
-- The Go model expects goal_type and current_value columns that don't exist in the schema

ALTER TABLE zuri_analytics.learning_goals
    ADD COLUMN IF NOT EXISTS goal_type VARCHAR(50) DEFAULT 'study_time',
    ADD COLUMN IF NOT EXISTS current_value INTEGER DEFAULT 0;
