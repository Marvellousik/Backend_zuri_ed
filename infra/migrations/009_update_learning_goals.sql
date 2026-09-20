-- Migration: Add current_value and goal_type to learning_goals
-- Enables auto-progress tracking and goal categorization

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.tables 
        WHERE table_schema = 'zuri_analytics' 
          AND table_name = 'learning_goals' 
          AND table_type = 'BASE TABLE'
    ) THEN
        ALTER TABLE zuri_analytics.learning_goals
            ADD COLUMN IF NOT EXISTS current_value INTEGER DEFAULT 0,
            ADD COLUMN IF NOT EXISTS goal_type VARCHAR(50) DEFAULT 'study_time';
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.tables 
        WHERE table_schema = 'analytics' 
          AND table_name = 'learning_goals' 
          AND table_type = 'BASE TABLE'
    ) THEN
        ALTER TABLE analytics.learning_goals
            ADD COLUMN IF NOT EXISTS current_value INTEGER DEFAULT 0,
            ADD COLUMN IF NOT EXISTS goal_type VARCHAR(50) DEFAULT 'study_time';
    END IF;
END $$;
