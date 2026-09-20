-- Migration: Create notification schema for Notification Service
-- This migration sets up tables for push notifications, emails, and scheduling

-- Create notification schema
CREATE SCHEMA IF NOT EXISTS zuri_notification;

-- Notification preferences per user
CREATE TABLE IF NOT EXISTS zuri_notification.preferences (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL UNIQUE REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    
    -- Push notifications
    push_enabled BOOLEAN DEFAULT true,
    push_device_tokens TEXT[], -- Array of FCM tokens
    
    -- Email notifications
    email_enabled BOOLEAN DEFAULT true,
    email_frequency VARCHAR(20) DEFAULT 'immediate', -- immediate, daily_digest, weekly_digest
    
    -- Quiet hours (no notifications during these hours)
    quiet_hours_start INTEGER DEFAULT 22, -- 10 PM
    quiet_hours_end INTEGER DEFAULT 8,    -- 8 AM
    timezone VARCHAR(50) DEFAULT 'UTC',
    
    -- Notification types (opt-in/out)
    notify_on_quiz_completion BOOLEAN DEFAULT true,
    notify_on_streak_achievement BOOLEAN DEFAULT true,
    notify_on_goal_completion BOOLEAN DEFAULT true,
    notify_on_material_processed BOOLEAN DEFAULT true,
    notify_on_study_reminder BOOLEAN DEFAULT true,
    
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create index on user_id
CREATE INDEX IF NOT EXISTS idx_notification_preferences_user_id ON zuri_notification.preferences(user_id);

-- Notification queue (for immediate sending)
CREATE TABLE IF NOT EXISTS zuri_notification.queue (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    
    -- Notification details
    notification_type VARCHAR(50) NOT NULL, -- push, email
    channel VARCHAR(50) NOT NULL, -- fcm, smtp
    
    -- Content
    title VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    data JSONB, -- Additional payload data
    
    -- Target
    device_token VARCHAR(500), -- For push notifications
    email_address VARCHAR(255), -- For email notifications
    
    -- Scheduling
    scheduled_at TIMESTAMP WITH TIME ZONE,
    sent_at TIMESTAMP WITH TIME ZONE,
    
    -- Status
    status VARCHAR(20) DEFAULT 'pending', -- pending, sent, failed, cancelled
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for notification queue
CREATE INDEX IF NOT EXISTS idx_notification_queue_user_id ON zuri_notification.queue(user_id);
CREATE INDEX IF NOT EXISTS idx_notification_queue_status ON zuri_notification.queue(status);
CREATE INDEX IF NOT EXISTS idx_notification_queue_scheduled ON zuri_notification.queue(scheduled_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_notification_queue_created ON zuri_notification.queue(created_at);

-- Notification history (sent notifications archive)
CREATE TABLE IF NOT EXISTS zuri_notification.history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    
    notification_type VARCHAR(50) NOT NULL,
    channel VARCHAR(50) NOT NULL,
    
    title VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    data JSONB,
    
    sent_at TIMESTAMP WITH TIME ZONE NOT NULL,
    delivered_at TIMESTAMP WITH TIME ZONE,
    opened_at TIMESTAMP WITH TIME ZONE,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for history
CREATE INDEX IF NOT EXISTS idx_notification_history_user_id ON zuri_notification.history(user_id);
CREATE INDEX IF NOT EXISTS idx_notification_history_sent ON zuri_notification.history(sent_at);

-- Scheduled reminders (for spaced repetition, study reminders)
CREATE TABLE IF NOT EXISTS zuri_notification.scheduled_reminders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    
    reminder_type VARCHAR(50) NOT NULL, -- study_reminder, quiz_reminder, review_reminder
    title VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,
    
    -- Scheduling
    scheduled_for TIMESTAMP WITH TIME ZONE NOT NULL,
    timezone VARCHAR(50) DEFAULT 'UTC',
    
    -- Recurrence (optional)
    recurrence VARCHAR(20), -- daily, weekly, none
    recurrence_end_date DATE,
    
    -- Related entity
    entity_type VARCHAR(50), -- course, quiz, material, topic
    entity_id UUID,
    
    -- Status
    is_active BOOLEAN DEFAULT true,
    sent_at TIMESTAMP WITH TIME ZONE,
    cancelled_at TIMESTAMP WITH TIME ZONE,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for scheduled reminders
CREATE INDEX IF NOT EXISTS idx_scheduled_reminders_user_id ON zuri_notification.scheduled_reminders(user_id);
CREATE INDEX IF NOT EXISTS idx_scheduled_reminders_scheduled ON zuri_notification.scheduled_reminders(scheduled_for) WHERE is_active = true AND sent_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_scheduled_reminders_entity ON zuri_notification.scheduled_reminders(entity_type, entity_id);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION zuri_notification.update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for automatically updating updated_at
DROP TRIGGER IF EXISTS update_notification_preferences_updated_at ON zuri_notification.preferences;
CREATE TRIGGER update_notification_preferences_updated_at
    BEFORE UPDATE ON zuri_notification.preferences
    FOR EACH ROW
    EXECUTE FUNCTION zuri_notification.update_updated_at_column();

DROP TRIGGER IF EXISTS update_scheduled_reminders_updated_at ON zuri_notification.scheduled_reminders;
CREATE TRIGGER update_scheduled_reminders_updated_at
    BEFORE UPDATE ON zuri_notification.scheduled_reminders
    FOR EACH ROW
    EXECUTE FUNCTION zuri_notification.update_updated_at_column();

-- View for pending notifications (to be picked up by worker)
CREATE OR REPLACE VIEW zuri_notification.pending_notifications AS
SELECT 
    q.*,
    p.push_enabled,
    p.email_enabled,
    p.quiet_hours_start,
    p.quiet_hours_end,
    p.timezone,
    u.email as user_email
FROM zuri_notification.queue q
JOIN zuri_notification.preferences p ON q.user_id = p.user_id
JOIN zuri_auth.users u ON q.user_id = u.id
WHERE q.status = 'pending'
AND q.scheduled_at <= CURRENT_TIMESTAMP;

-- View for active reminders due for sending
CREATE OR REPLACE VIEW zuri_notification.due_reminders AS
SELECT 
    r.*,
    p.timezone as user_timezone,
    p.push_enabled,
    p.email_enabled
FROM zuri_notification.scheduled_reminders r
JOIN zuri_notification.preferences p ON r.user_id = p.user_id
WHERE r.is_active = true
AND r.sent_at IS NULL
AND r.scheduled_for <= CURRENT_TIMESTAMP;
