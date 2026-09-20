-- Migration: Create sync schema for Sync Service
-- This migration sets up tables for real-time synchronization
-- Unified canonical schema: zuri_sync with backward-compatible sync views

-- Create schemas
CREATE SCHEMA IF NOT EXISTS zuri_sync;
CREATE SCHEMA IF NOT EXISTS sync;

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

-- Create indexes for sync events
CREATE INDEX IF NOT EXISTS idx_sync_events_user_id ON zuri_sync.events(user_id);
CREATE INDEX IF NOT EXISTS idx_sync_events_course_id ON zuri_sync.events(course_id);
CREATE INDEX IF NOT EXISTS idx_sync_events_type ON zuri_sync.events(event_type);
CREATE INDEX IF NOT EXISTS idx_sync_events_created ON zuri_sync.events(created_at);

-- User presence (online status and active session info)
CREATE TABLE IF NOT EXISTS zuri_sync.presence (
    user_id UUID PRIMARY KEY REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    
    status VARCHAR(20) DEFAULT 'offline', -- online, away, busy, offline
    status_message VARCHAR(255),
    
    -- Current activity
    current_course_id UUID,
    current_material_id UUID,
    activity_details JSONB, -- Current reading position, quiz in progress, etc.
    
    -- Connection tracking
    active_connections INTEGER DEFAULT 0,
    last_seen_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_activity_type VARCHAR(50),
    
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for presence
CREATE INDEX IF NOT EXISTS idx_sync_presence_status ON zuri_sync.presence(status) WHERE status != 'offline';
CREATE INDEX IF NOT EXISTS idx_sync_presence_course ON zuri_sync.presence(current_course_id);
CREATE INDEX IF NOT EXISTS idx_sync_presence_last_seen ON zuri_sync.presence(last_seen_at);

-- Device sync state (tracks what each device has synced)
CREATE TABLE IF NOT EXISTS zuri_sync.device_state (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    device_id VARCHAR(255) NOT NULL,
    
    -- Last synced timestamps per entity
    last_sync_materials TIMESTAMP WITH TIME ZONE,
    last_sync_quizzes TIMESTAMP WITH TIME ZONE,
    last_sync_flashcards TIMESTAMP WITH TIME ZONE,
    last_sync_analytics TIMESTAMP WITH TIME ZONE,
    
    -- Client version info
    app_version VARCHAR(50),
    os_version VARCHAR(50),
    
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(user_id, device_id)
);

-- Create indexes for device state
CREATE INDEX IF NOT EXISTS idx_sync_device_state_user ON zuri_sync.device_state(user_id);
CREATE INDEX IF NOT EXISTS idx_sync_device_state_device ON zuri_sync.device_state(device_id);

-- Change log for delta sync (CDC - Change Data Capture)
CREATE TABLE IF NOT EXISTS zuri_sync.change_log (
    id BIGSERIAL PRIMARY KEY,
    
    -- What changed
    table_name VARCHAR(100) NOT NULL,
    record_id UUID NOT NULL,
    operation VARCHAR(10) NOT NULL, -- INSERT, UPDATE, DELETE
    
    -- Scope
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    course_id UUID,
    
    -- The actual changes
    changed_fields JSONB, -- Array of field names that changed
    old_data JSONB,       -- Previous values (for conflict resolution)
    new_data JSONB,       -- New values
    
    -- Metadata
    version INTEGER DEFAULT 1,
    source_device_id VARCHAR(255),
    
    changed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    processed BOOLEAN DEFAULT false
);

-- Create indexes for change log
CREATE INDEX IF NOT EXISTS idx_sync_change_log_user ON zuri_sync.change_log(user_id, changed_at);
CREATE INDEX IF NOT EXISTS idx_sync_change_log_table ON zuri_sync.change_log(table_name, record_id);
CREATE INDEX IF NOT EXISTS idx_sync_change_log_unprocessed ON zuri_sync.change_log(processed) WHERE processed = false;

-- Offline sync queue (changes made while offline to be merged)
CREATE TABLE IF NOT EXISTS zuri_sync.offline_queue (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    device_id VARCHAR(255) NOT NULL,
    
    -- Operation details
    operation_type VARCHAR(50) NOT NULL,
    target_entity VARCHAR(100) NOT NULL,
    entity_id UUID,
    payload JSONB NOT NULL,
    
    -- Client-side timestamp
    client_timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    
    -- Conflict resolution
    client_version INTEGER DEFAULT 1,
    status VARCHAR(20) DEFAULT 'pending', -- pending, applied, conflicted, failed
    conflict_resolution VARCHAR(20),      -- client_wins, server_wins, manual
    error_message TEXT,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes for offline queue
CREATE INDEX IF NOT EXISTS idx_sync_offline_queue_pending ON zuri_sync.offline_queue(user_id, status) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_sync_offline_queue_device ON zuri_sync.offline_queue(device_id);

-- Function to update presence updated_at
CREATE OR REPLACE FUNCTION zuri_sync.update_presence_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

DROP TRIGGER IF EXISTS update_sync_presence_timestamp ON zuri_sync.presence;
CREATE TRIGGER update_sync_presence_timestamp
    BEFORE UPDATE ON zuri_sync.presence
    FOR EACH ROW
    EXECUTE FUNCTION zuri_sync.update_presence_timestamp();

-- View for online users summary
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

-- Backward compatibility views for legacy or external callers expecting sync.*
CREATE OR REPLACE VIEW sync.connections AS SELECT * FROM zuri_sync.connections;
CREATE OR REPLACE VIEW sync.events AS SELECT * FROM zuri_sync.events;
CREATE OR REPLACE VIEW sync.presence AS SELECT * FROM zuri_sync.presence;
CREATE OR REPLACE VIEW sync.device_state AS SELECT * FROM zuri_sync.device_state;
CREATE OR REPLACE VIEW sync.change_log AS SELECT * FROM zuri_sync.change_log;
CREATE OR REPLACE VIEW sync.offline_queue AS SELECT * FROM zuri_sync.offline_queue;
CREATE OR REPLACE VIEW sync.online_users AS SELECT * FROM zuri_sync.online_users;
CREATE OR REPLACE VIEW sync.pending_changes AS SELECT * FROM zuri_sync.pending_changes;
