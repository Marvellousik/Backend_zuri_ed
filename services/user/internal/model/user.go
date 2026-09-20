// Package model contains domain models for the User Service.
package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a user in the system.
type User struct {
	ID                   uuid.UUID      `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	Email                string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	PasswordHash         string         `gorm:"type:varchar(255);not null" json:"-"`
	FirstName            string         `gorm:"type:varchar(100)" json:"first_name"`
	LastName             string         `gorm:"type:varchar(100)" json:"last_name"`
	School               string         `gorm:"type:varchar(255)" json:"school"`
	Department           string         `gorm:"type:varchar(255)" json:"department"`
	AcademicLevel        string         `gorm:"type:varchar(50)" json:"academic_level"`
	Timezone             string         `gorm:"type:varchar(50);default:'UTC'" json:"timezone"`
	EmailVerified        bool           `gorm:"default:false" json:"email_verified"`
	VerificationCode     string         `gorm:"type:varchar(10)" json:"-"`
	VerificationCodeExpiresAt *time.Time `json:"-"`
	IsActive             bool           `gorm:"default:true" json:"is_active"`
	Role                 string         `gorm:"type:varchar(20);default:'student'" json:"role"`
	InstitutionID        string         `gorm:"type:varchar(255);index" json:"institution_id,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`

	// Associations
	Sessions      []UserSession   `json:"sessions,omitempty"`
	RefreshTokens []RefreshToken  `json:"-"`
}

// Role constants for user roles
const (
	RoleStudent    = "student"
	RoleInstructor = "instructor"
	RoleAdmin      = "admin"
	RoleSuperAdmin = "super_admin"
)

// IsAdmin returns true if user has admin or super_admin role
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin || u.Role == RoleSuperAdmin
}

// IsSuperAdmin returns true if user has super_admin role
func (u *User) IsSuperAdmin() bool {
	return u.Role == RoleSuperAdmin
}

// TableName specifies the table name for User.
func (User) TableName() string {
	return "zuri_auth.users"
}

// FullName returns the user's full name.
func (u *User) FullName() string {
	if u.FirstName == "" && u.LastName == "" {
		return ""
	}
	return u.FirstName + " " + u.LastName
}

// IsVerified returns true if the user's email is verified.
func (u *User) IsVerified() bool {
	return u.EmailVerified
}

// RefreshToken represents a refresh token for session management.
type RefreshToken struct {
	ID             uuid.UUID      `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID         uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash      string         `gorm:"type:varchar(255);not null;index" json:"-"`
	DeviceInfo     DeviceInfo     `gorm:"type:jsonb" json:"device_info"`
	IPAddress      string         `gorm:"type:inet" json:"ip_address"`
	IssuedAt       time.Time      `gorm:"default:CURRENT_TIMESTAMP" json:"issued_at"`
	ExpiresAt      time.Time      `gorm:"not null" json:"expires_at"`
	RevokedAt      *time.Time     `json:"revoked_at,omitempty"`
	Revoked        bool           `gorm:"default:false;index" json:"revoked"`
	ReplacedByTokenID *uuid.UUID  `gorm:"type:uuid" json:"-"`
	ReplacedByToken   *RefreshToken `gorm:"foreignKey:ReplacedByTokenID" json:"-"`
	CreatedAt      time.Time      `json:"created_at"`

	// Associations
	User        User         `gorm:"foreignKey:UserID" json:"-"`
	UserSession *UserSession `gorm:"foreignKey:RefreshTokenID" json:"-"`
}

// TableName specifies the table name for RefreshToken.
func (RefreshToken) TableName() string {
	return "zuri_auth.refresh_tokens"
}

// IsExpired returns true if the token has expired.
func (rt *RefreshToken) IsExpired() bool {
	return time.Now().After(rt.ExpiresAt)
}

// IsRevoked returns true if the token has been revoked.
func (rt *RefreshToken) IsRevoked() bool {
	return rt.Revoked || rt.RevokedAt != nil
}

// CanBeUsed returns true if the token is valid for use.
func (rt *RefreshToken) CanBeUsed() bool {
	return !rt.IsExpired() && !rt.IsRevoked()
}

// DeviceInfo holds information about the device used.
type DeviceInfo struct {
	DeviceName string `json:"device_name,omitempty"`
	DeviceType string `json:"device_type,omitempty"` // mobile, tablet, desktop
	OS         string `json:"os,omitempty"`
	OSVersion  string `json:"os_version,omitempty"`
	Browser    string `json:"browser,omitempty"`
	BrowserVersion string `json:"browser_version,omitempty"`
}

// Value implements the driver.Valuer interface.
func (di DeviceInfo) Value() (driver.Value, error) {
	return json.Marshal(di)
}

// Scan implements the sql.Scanner interface.
func (di *DeviceInfo) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return fmt.Errorf("cannot scan type %T into DeviceInfo", value)
	}
	return json.Unmarshal(bytes, di)
}

// UserSession represents a user session for device management.
type UserSession struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID         uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	RefreshTokenID *uuid.UUID `gorm:"type:uuid" json:"refresh_token_id,omitempty"`
	DeviceName     string     `gorm:"type:varchar(255)" json:"device_name"`
	DeviceType     string     `gorm:"type:varchar(50)" json:"device_type"`
	OS             string     `gorm:"type:varchar(100)" json:"os"`
	Browser        string     `gorm:"type:varchar(100)" json:"browser"`
	IPAddress      string     `gorm:"type:inet" json:"ip_address"`
	Location       string     `gorm:"type:varchar(255)" json:"location"`
	LastActiveAt   time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"last_active_at"`
	CreatedAt      time.Time  `json:"created_at"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`

	// Associations
	User         User          `gorm:"foreignKey:UserID" json:"-"`
	RefreshToken *RefreshToken `gorm:"foreignKey:RefreshTokenID" json:"-"`
}

// TableName specifies the table name for UserSession.
func (UserSession) TableName() string {
	return "zuri_auth.user_sessions"
}

// IsActive returns true if the session is still active.
func (us *UserSession) IsActive() bool {
	return us.RevokedAt == nil
}

// JWTKey represents a stored JWT key pair.
type JWTKey struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	KeyType   string     `gorm:"type:varchar(20);not null" json:"key_type"` // private, public
	KeyData   string     `gorm:"type:text;not null" json:"-"`
	Algorithm string     `gorm:"type:varchar(20);default:'RS256'" json:"algorithm"`
	IsActive  bool       `gorm:"default:true;index" json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	RotatedAt *time.Time `json:"rotated_at,omitempty"`
}

// TableName specifies the table name for JWTKey.
func (JWTKey) TableName() string {
	return "zuri_auth.jwt_keys"
}

// PasswordReset represents a password reset request.
type PasswordReset struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	TokenHash string     `gorm:"type:varchar(255);not null;index" json:"-"`
	ExpiresAt time.Time  `gorm:"not null" json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	Used      bool       `gorm:"default:false" json:"used"`
	CreatedAt time.Time  `json:"created_at"`

	// Associations
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// TableName specifies the table name for PasswordReset.
func (PasswordReset) TableName() string {
	return "zuri_auth.password_resets"
}

// IsExpired returns true if the reset token has expired.
func (pr *PasswordReset) IsExpired() bool {
	return time.Now().After(pr.ExpiresAt)
}

// CanBeUsed returns true if the reset token is valid for use.
func (pr *PasswordReset) CanBeUsed() bool {
	return !pr.IsExpired() && !pr.Used
}

// TokenBlacklist represents a blacklisted access token.
type TokenBlacklist struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	TokenJTI  string    `gorm:"type:varchar(255);not null;index" json:"token_jti"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	ExpiresAt time.Time `gorm:"not null;index" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName specifies the table name for TokenBlacklist.
func (TokenBlacklist) TableName() string {
	return "zuri_auth.token_blacklist"
}

// Role represents a canonical role within the auth domain.
type Role struct {
	ID          string    `gorm:"type:varchar(50);primary_key" json:"id"`
	Name        string    `gorm:"type:varchar(100);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName specifies the table name for Role.
func (Role) TableName() string {
	return "zuri_auth.roles"
}

// Permission represents a system capability/action on a resource.
type Permission struct {
	ID          string    `gorm:"type:varchar(100);primary_key" json:"id"`
	Name        string    `gorm:"type:varchar(100);not null" json:"name"`
	Resource    string    `gorm:"type:varchar(50);not null" json:"resource"`
	Action      string    `gorm:"type:varchar(50);not null" json:"action"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName specifies the table name for Permission.
func (Permission) TableName() string {
	return "zuri_auth.permissions"
}

// InstitutionMembership associates a User with an Institution and Role.
type InstitutionMembership struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()" json:"id"`
	UserID        uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	InstitutionID string    `gorm:"type:varchar(255);not null" json:"institution_id"`
	Role          string    `gorm:"type:varchar(50);not null;default:'student'" json:"role"`
	DepartmentID  string    `gorm:"type:varchar(255)" json:"department_id,omitempty"`
	Identifier    string    `gorm:"type:varchar(100)" json:"identifier,omitempty"` // Matric or Staff ID
	IsDefault     bool      `gorm:"default:false" json:"is_default"`
	Status        string    `gorm:"type:varchar(50);default:'active'" json:"status"` // active, pending, suspended
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// Associations
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// TableName specifies the table name for InstitutionMembership.
func (InstitutionMembership) TableName() string {
	return "zuri_auth.institution_memberships"
}
