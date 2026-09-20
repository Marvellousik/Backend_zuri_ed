-- Migration: Create sync schema for Sync Service
-- This migration sets up tables for real-time synchronization

-- Create sync schema
CREATE SCHEMA IF NOT EXISTS zuri_sync;

-- WebSocket connections (active sessions)
CREATE TABLE IF NOT EXISTS zuri_sync.connections (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    
    -- Connection details
    connection_id VARCHAR(255) NOT NULL UNIQUE, -- WebSocket connection ID
    device_id VARCHAR(255), -- Device identifier
    device_type VARCHAR(50), -- mobile, tablet, desktop, web
    
    -- Connection state
    is_active BOOLEAN DEFAULT true,
    connected_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_ping_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    disconnected_at TIMESTAMP WITH TIME ZONE,
    
    -- Client info
    ip_address INET,
    user_agent TEXT,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for connections
CREATE INDEX IF NOT EXISTS idx_sync_connections_user_id ON zuri_sync.connections(user_id);
CREATE INDEX IF NOT EXISTS idx_sync_connections_active ON zuri_sync.connections(user_id, is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_sync_connections_connection ON zuri_sync.connections(connection_id);

-- Sync events (for real-time broadcasting)
CREATE TABLE IF NOT EXISTS zuri_sync.events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- Event metadata
    event_type VARCHAR(50) NOT NULL, -- material.created, quiz.completed, etc.
    event_name VARCHAR(100) NOT NULL, -- Human-readable event name
    
    -- Target (who should receive this)
    user_id UUID REFERENCES zuri_auth.users(id) ON DELETE CASCADE, -- NULL = broadcast to all
    course_id UUID, -- Optional: specific to a course
    
    -- Event payload
    payload JSONB NOT NULL,
    
    -- Source
    source_service VARCHAR(50) NOT NULL, -- Which service generated this
    source_connection_id VARCHAR(255), -- Which connection (if any)
    
    -- Status
    is_broadcast BOOLEAN DEFAULT false, -- True = sent to all user devices
    processed_at TIMESTAMP WITH TIME ZONE,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for events
CREATE INDEX IF NOT EXISTS idx_sync_events_user_id ON zuri_sync.events(user_id);
CREATE INDEX IF NOT EXISTS idx_sync_events_type ON zuri_sync.events(event_type);
CREATE INDEX IF NOT EXISTS idx_sync_events_created ON zuri_sync.events(created_at);
CREATE INDEX IF NOT EXISTS idx_sync_events_unprocessed ON zuri_sync.events(processed_at) WHERE processed_at IS NULL;

-- Device sync state (for tracking what each device has seen)
CREATE TABLE IF NOT EXISTS zuri_sync.device_state (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    device_id VARCHAR(255) NOT NULL,
    
    -- Last event seen by this device
    last_event_id UUID,
    last_event_timestamp TIMESTAMP WITH TIME ZONE,
    
    -- Sync cursor for pagination
    sync_cursor VARCHAR(255),
    
    -- Device sync status
    is_syncing BOOLEAN DEFAULT false,
    last_sync_at TIMESTAMP WITH TIME ZONE,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id, device_id)
);

-- Create indexes for device state
CREATE INDEX IF NOT EXISTS idx_sync_device_state_user ON zuri_sync.device_state(user_id);
CREATE INDEX IF NOT EXISTS idx_sync_device_state_device ON zuri_sync.device_state(device_id);

-- Change log (for CDC - Change Data Capture)
-- This table captures all database changes for sync purposes
CREATE TABLE IF NOT EXISTS zuri_sync.change_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- Change metadata
    table_name VARCHAR(100) NOT NULL, -- Which table changed
    operation VARCHAR(10) NOT NULL, -- INSERT, UPDATE, DELETE
    
    -- Record identification
    record_id UUID NOT NULL, -- Primary key of changed record
    user_id UUID, -- Owner of the record (if applicable)
    
    -- Change data
    old_data JSONB, -- Previous state (for UPDATE/DELETE)
    new_data JSONB, -- New state (for INSERT/UPDATE)
    
    -- Change timestamp
    changed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Sync status
    processed BOOLEAN DEFAULT false,
    processed_at TIMESTAMP WITH TIME ZONE,
    broadcast_to TEXT[] -- Array of connection IDs that received this
);

-- Create indexes for change log
CREATE INDEX IF NOT EXISTS idx_sync_change_log_table ON zuri_sync.change_log(table_name);
CREATE INDEX IF NOT EXISTS idx_sync_change_log_record ON zuri_sync.change_log(record_id);
CREATE INDEX IF NOT EXISTS idx_sync_change_log_user ON zuri_sync.change_log(user_id);
CREATE INDEX IF NOT EXISTS idx_sync_change_log_processed ON zuri_sync.change_log(processed) WHERE processed = false;
CREATE INDEX IF NOT EXISTS idx_sync_change_log_timestamp ON zuri_sync.change_log(changed_at);

-- Presence tracking (who's online)
CREATE TABLE IF NOT EXISTS zuri_sync.presence (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    
    -- Status
    status VARCHAR(20) DEFAULT 'offline', -- online, away, offline, busy
    status_message VARCHAR(255),
    
    -- Last activity
    last_seen_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_activity_type VARCHAR(50), -- viewing_course, taking_quiz, etc.
    last_activity_data JSONB,
    
    -- Active connections count
    active_connections INTEGER DEFAULT 0,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id)
);

-- Create indexes for presence
CREATE INDEX IF NOT EXISTS idx_sync_presence_user ON zuri_sync.presence(user_id);
CREATE INDEX IF NOT EXISTS idx_sync_presence_status ON zuri_sync.presence(status);
CREATE INDEX IF NOT EXISTS idx_sync_presence_last_seen ON zuri_sync.presence(last_seen_at);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION zuri_sync.update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for automatically updating updated_at
DROP TRIGGER IF EXISTS update_device_state_updated_at ON zuri_sync.device_state;
CREATE TRIGGER update_device_state_updated_at
    BEFORE UPDATE ON zuri_sync.device_state
    FOR EACH ROW
    EXECUTE FUNCTION zuri_sync.update_updated_at_column();

DROP TRIGGER IF EXISTS update_presence_updated_at ON zuri_sync.presence;
CREATE TRIGGER update_presence_updated_at
    BEFORE UPDATE ON zuri_sync.presence
    FOR EACH ROW
    EXECUTE FUNCTION zuri_sync.update_updated_at_column();

-- View for active users (online status)
CREATE OR REPLACE VIEW zuri_sync.online_users AS
SELECT 
    p.user_id,
    p.status,
    p.status_message,
    p.last_seen_at,
    p.last_activity_type,
    COUNT(c.id) as connection_count
FROM zuri_sync.presence p
LEFT JOIN zuri_sync.connections c ON p.user_id = c.user_id AND c.is_active = true
WHERE p.status != 'offline'
GROUP BY p.user_id, p.status, p.status_message, p.last_seen_at, p.last_activity_type;

-- View for pending changes to sync
CREATE OR REPLACE VIEW zuri_sync.pending_changes AS
SELECT 
    cl.*,
    u.email as user_email
FROM zuri_sync.change_log cl
LEFT JOIN zuri_auth.users u ON cl.user_id = u.id
WHERE cl.processed = false
ORDER BY cl.changed_at ASC;
