-- Migration: 011_create_institution_memberships.sql
-- Description: Institutional Memberships, Roles, Permissions, and Multi-Tenant Role Isolation

CREATE SCHEMA IF NOT EXISTS zuri_auth;
CREATE SCHEMA IF NOT EXISTS auth;

-- 1. Canonical Roles Table
CREATE TABLE IF NOT EXISTS zuri_auth.roles (
    id VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Seed Canonical Roles
INSERT INTO zuri_auth.roles (id, name, description) VALUES
    ('student', 'Student', 'Enrolled student pursuing academic courses and degrees'),
    ('lecturer', 'Lecturer', 'Academic course instructor, lecturer, and grader'),
    ('instructor', 'Instructor', 'Content creator and teaching assistant'),
    ('department_admin', 'Department Administrator', 'Administrator managing departmental courses and staff'),
    ('faculty_admin', 'Faculty Administrator', 'Administrator managing faculty affairs and departments'),
    ('institution_admin', 'Institution Administrator', 'University executive managing institution configuration'),
    ('super_admin', 'Super Administrator', 'Global platform administrator')
ON CONFLICT (id) DO NOTHING;

-- 2. Canonical Permissions Table
CREATE TABLE IF NOT EXISTS zuri_auth.permissions (
    id VARCHAR(100) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    resource VARCHAR(50) NOT NULL,
    action VARCHAR(50) NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Seed Base Permissions
INSERT INTO zuri_auth.permissions (id, name, resource, action, description) VALUES
    ('course:read', 'Read Courses', 'course', 'read', 'View course details, materials, and syllabi'),
    ('course:write', 'Modify Courses', 'course', 'write', 'Create, update, and manage course offerings'),
    ('material:read', 'Read Materials', 'material', 'read', 'View and download course documents and media'),
    ('material:upload', 'Upload Materials', 'material', 'upload', 'Upload documents and lecture media'),
    ('lecture:manage', 'Manage Lectures', 'lecture', 'manage', 'Schedule and edit lecture sessions and summaries'),
    ('grade:view', 'View Grades', 'grade', 'view', 'View student grades and analytics'),
    ('grade:submit', 'Submit Grades', 'grade', 'submit', 'Grade assessments and submit scores'),
    ('institution:admin', 'Administer Institution', 'institution', 'admin', 'Manage institution settings and memberships')
ON CONFLICT (id) DO NOTHING;

-- 3. Role Permissions Mapping Table
CREATE TABLE IF NOT EXISTS zuri_auth.role_permissions (
    role_id VARCHAR(50) NOT NULL REFERENCES zuri_auth.roles(id) ON DELETE CASCADE,
    permission_id VARCHAR(100) NOT NULL REFERENCES zuri_auth.permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- Seed Role Permissions Mapping
INSERT INTO zuri_auth.role_permissions (role_id, permission_id) VALUES
    ('student', 'course:read'),
    ('student', 'material:read'),
    ('lecturer', 'course:read'),
    ('lecturer', 'course:write'),
    ('lecturer', 'material:read'),
    ('lecturer', 'material:upload'),
    ('lecturer', 'lecture:manage'),
    ('lecturer', 'grade:view'),
    ('lecturer', 'grade:submit'),
    ('instructor', 'course:read'),
    ('instructor', 'material:read'),
    ('instructor', 'material:upload'),
    ('instructor', 'grade:view'),
    ('department_admin', 'course:read'),
    ('department_admin', 'course:write'),
    ('department_admin', 'material:read'),
    ('department_admin', 'lecture:manage'),
    ('department_admin', 'grade:view'),
    ('faculty_admin', 'course:read'),
    ('faculty_admin', 'course:write'),
    ('faculty_admin', 'grade:view'),
    ('institution_admin', 'course:read'),
    ('institution_admin', 'course:write'),
    ('institution_admin', 'grade:view'),
    ('institution_admin', 'institution:admin'),
    ('super_admin', 'course:read'),
    ('super_admin', 'course:write'),
    ('super_admin', 'material:read'),
    ('super_admin', 'material:upload'),
    ('super_admin', 'lecture:manage'),
    ('super_admin', 'grade:view'),
    ('super_admin', 'grade:submit'),
    ('super_admin', 'institution:admin')
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- 4. Institution Memberships Table
CREATE TABLE IF NOT EXISTS zuri_auth.institution_memberships (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES zuri_auth.users(id) ON DELETE CASCADE,
    institution_id VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'student' REFERENCES zuri_auth.roles(id),
    department_id VARCHAR(255),
    identifier VARCHAR(100),
    is_default BOOLEAN DEFAULT FALSE,
    status VARCHAR(50) DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_user_institution UNIQUE (user_id, institution_id)
);

CREATE INDEX IF NOT EXISTS idx_memberships_user ON zuri_auth.institution_memberships(user_id);
CREATE INDEX IF NOT EXISTS idx_memberships_inst_role ON zuri_auth.institution_memberships(institution_id, role);
CREATE INDEX IF NOT EXISTS idx_memberships_status ON zuri_auth.institution_memberships(user_id, status);

-- 5. Backward Compatibility Views
CREATE OR REPLACE VIEW auth.roles AS SELECT * FROM zuri_auth.roles;
CREATE OR REPLACE VIEW auth.permissions AS SELECT * FROM zuri_auth.permissions;
CREATE OR REPLACE VIEW auth.role_permissions AS SELECT * FROM zuri_auth.role_permissions;
CREATE OR REPLACE VIEW auth.institution_memberships AS SELECT * FROM zuri_auth.institution_memberships;

-- 6. Backfill existing user institution bindings
INSERT INTO zuri_auth.institution_memberships (user_id, institution_id, role, is_default, status)
SELECT id, institution_id, COALESCE(role, 'student'), TRUE, 'active'
FROM zuri_auth.users
WHERE institution_id IS NOT NULL AND institution_id != ''
ON CONFLICT (user_id, institution_id) DO NOTHING;
