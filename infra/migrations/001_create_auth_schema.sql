-- Migration: Create auth schema and user-related tables
-- Unified canonical schema: zuri_auth with backward-compatible auth views

-- Create schemas
CREATE SCHEMA IF NOT EXISTS zuri_auth;
CREATE SCHEMA IF NOT EXISTS auth;

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table
CREATE TABLE IF NOT EXISTS zuri_auth.users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    school VARCHAR(255),
    department VARCHAR(255),
    academic_level VARCHAR(50), -- undergraduate, graduate, phd, etc.
    timezone VARCHAR(50) DEFAULT 'UTC',
    email_verified BOOLEAN DEFAULT FALSE,
    verification_code VARCHAR(10),
    verification_code_expires_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN DEFAULT TRUE,
    role VARCHAR(20) DEFAULT 'student',
    institution_id VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create index on email and tenant scoping for faster lookups
CREATE INDEX IF NOT EXISTS idx_users_email ON zuri_auth.users(email);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON zuri_auth.users(deleted_at);
CREATE INDEX IF NOT EXISTS idx_users_role ON zuri_auth.users(role);
CREATE INDEX IF NOT EXISTS idx_users_institution_id ON zuri_auth.users(institution_id);

-- Refresh tokens table
CREATE TABLE IF NOT EXISTS zuri_auth.refresh_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    device_info JSONB, -- JSON with device name, OS, browser
    ip_address INET,
    issued_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked_at TIMESTAMP WITH TIME ZONE,
    revoked BOOLEAN DEFAULT FALSE,
    replaced_by_token_id UUID REFERENCES zuri_auth.refresh_tokens(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for refresh tokens
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON zuri_auth.refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash ON zuri_auth.refresh_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_revoked ON zuri_auth.refresh_tokens(revoked);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON zuri_auth.refresh_tokens(expires_at);

-- JWT key storage (for the private key)
CREATE TABLE IF NOT EXISTS zuri_auth.jwt_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key_type VARCHAR(20) NOT NULL CHECK (key_type IN ('private', 'public')),
    key_data TEXT NOT NULL, -- Encrypted private key or plain public key
    algorithm VARCHAR(20) DEFAULT 'RS256',
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    rotated_at TIMESTAMP WITH TIME ZONE
);

-- Create index on active keys
CREATE INDEX IF NOT EXISTS idx_jwt_keys_active ON zuri_auth.jwt_keys(is_active) WHERE is_active = TRUE;

-- Password reset tokens table
CREATE TABLE IF NOT EXISTS zuri_auth.password_resets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    used_at TIMESTAMP WITH TIME ZONE,
    used BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_password_resets_user_id ON zuri_auth.password_resets(user_id);
CREATE INDEX IF NOT EXISTS idx_password_resets_token_hash ON zuri_auth.password_resets(token_hash);

-- Token blacklist for revoked access tokens (until they naturally expire)
CREATE TABLE IF NOT EXISTS zuri_auth.token_blacklist (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    token_jti VARCHAR(255) NOT NULL, -- JWT ID
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_token_blacklist_jti ON zuri_auth.token_blacklist(token_jti);
CREATE INDEX IF NOT EXISTS idx_token_blacklist_expires_at ON zuri_auth.token_blacklist(expires_at);

-- User sessions for device management
CREATE TABLE IF NOT EXISTS zuri_auth.user_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    refresh_token_id UUID REFERENCES zuri_auth.refresh_tokens(id) ON DELETE SET NULL,
    device_name VARCHAR(255),
    device_type VARCHAR(50), -- mobile, tablet, desktop, etc.
    os VARCHAR(100),
    browser VARCHAR(100),
    ip_address INET,
    location VARCHAR(255),
    last_active_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON zuri_auth.user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_revoked_at ON zuri_auth.user_sessions(revoked_at);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION zuri_auth.update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger to automatically update updated_at
DROP TRIGGER IF EXISTS update_users_updated_at ON zuri_auth.users;
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON zuri_auth.users
    FOR EACH ROW
    EXECUTE FUNCTION zuri_auth.update_updated_at_column();

-- Cleanup old blacklisted tokens (run periodically)
CREATE OR REPLACE FUNCTION zuri_auth.cleanup_expired_blacklisted_tokens()
RETURNS void AS $$
BEGIN
    DELETE FROM zuri_auth.token_blacklist WHERE expires_at < CURRENT_TIMESTAMP;
    DELETE FROM zuri_auth.password_resets WHERE expires_at < CURRENT_TIMESTAMP AND used = FALSE;
    DELETE FROM zuri_auth.refresh_tokens WHERE expires_at < CURRENT_TIMESTAMP AND revoked = TRUE;
END;
$$ LANGUAGE plpgsql;

-- Backward compatibility views for legacy or external callers expecting auth.*
CREATE OR REPLACE VIEW auth.users AS SELECT * FROM zuri_auth.users;
CREATE OR REPLACE VIEW auth.refresh_tokens AS SELECT * FROM zuri_auth.refresh_tokens;
CREATE OR REPLACE VIEW auth.jwt_keys AS SELECT * FROM zuri_auth.jwt_keys;
CREATE OR REPLACE VIEW auth.password_resets AS SELECT * FROM zuri_auth.password_resets;
CREATE OR REPLACE VIEW auth.token_blacklist AS SELECT * FROM zuri_auth.token_blacklist;
CREATE OR REPLACE VIEW auth.user_sessions AS SELECT * FROM zuri_auth.user_sessions;
