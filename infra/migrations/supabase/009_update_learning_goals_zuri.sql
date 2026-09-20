-- Migration: Add current_value and goal_type to learning_goals (zuri schema)
-- Enables auto-progress tracking and goal categorization

ALTER TABLE zuri_analytics.learning_goals
    ADD COLUMN IF NOT EXISTS current_value INTEGER DEFAULT 0,
    ADD COLUMN IF NOT EXISTS goal_type VARCHAR(50) DEFAULT 'study_time';
