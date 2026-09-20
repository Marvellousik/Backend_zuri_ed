// Package auth provides JWT validation middleware and password hashing helpers.
package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidToken is returned when a token is invalid.
	ErrInvalidToken = errors.New("invalid token")
	// ErrTokenExpired is returned when a token has expired.
	ErrTokenExpired = errors.New("token expired")
	// ErrTokenRevoked is returned when a token has been revoked.
	ErrTokenRevoked = errors.New("token revoked")
	// ErrInvalidPassword is returned when a password is incorrect.
	ErrInvalidPassword = errors.New("invalid password")
)

// Claims represents JWT claims.
type Claims struct {
	UserID        string `json:"user_id"`
	Email         string `json:"email"`
	Role          string `json:"role"`
	TokenType     string `json:"token_type"`
	InstitutionID string `json:"institution_id,omitempty"`
	jwt.RegisteredClaims
}

// TokenPair represents an access token and refresh token pair.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// JWTManager handles JWT creation and validation.
type JWTManager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	signMethod jwt.SigningMethod
}

// NewJWTManager creates a new JWT manager with the provided keys.
func NewJWTManager(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) *JWTManager {
	return &JWTManager{
		privateKey: privateKey,
		publicKey:  publicKey,
		signMethod: jwt.SigningMethodRS256,
	}
}

// NewJWTManagerWithPublicKeyOnly creates a JWT manager for validation only (Gateway use case).
func NewJWTManagerWithPublicKeyOnly(publicKey *rsa.PublicKey) *JWTManager {
	return &JWTManager{
		publicKey:  publicKey,
		signMethod: jwt.SigningMethodRS256,
	}
}

// GenerateTokenPair generates a new access and refresh token pair.
func (m *JWTManager) GenerateTokenPair(userID, email, role string, accessTTL, refreshTTL time.Duration) (*TokenPair, error) {
	return m.GenerateTenantTokenPair(userID, email, role, "", accessTTL, refreshTTL)
}

// GenerateTenantTokenPair generates a new access and refresh token pair with institution tenant ID.
func (m *JWTManager) GenerateTenantTokenPair(userID, email, role, institutionID string, accessTTL, refreshTTL time.Duration) (*TokenPair, error) {
	now := time.Now()

	// Generate access token
	accessClaims := Claims{
		UserID:        userID,
		Email:         email,
		Role:          role,
		TokenType:     "access",
		InstitutionID: institutionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Subject:   userID,
			ID:        uuid.New().String(),
		},
	}

	accessToken, err := jwt.NewWithClaims(m.signMethod, accessClaims).SignedString(m.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Generate refresh token
	refreshTokenBytes := make([]byte, 32)
	if _, err := rand.Read(refreshTokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}
	refreshToken := base64.URLEncoding.EncodeToString(refreshTokenBytes)

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    now.Add(accessTTL),
		TokenType:    "Bearer",
	}, nil
}

// ValidateToken validates a JWT token and returns the claims.
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.publicKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// Ensure it's an access token
	if claims.TokenType != "access" {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// HashRefreshToken hashes a refresh token using SHA-256.
func HashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(hash[:])
}

// GenerateVerificationCode generates a 6-digit verification code.
func GenerateVerificationCode() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based generation
		return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	// Convert to 6-digit number
	code := int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	return fmt.Sprintf("%06d", code%1000000)
}

// PasswordHasher handles password hashing and verification.
type PasswordHasher struct {
	cost int
}

// NewPasswordHasher creates a new password hasher with the specified cost.
func NewPasswordHasher(cost int) *PasswordHasher {
	return &PasswordHasher{cost: cost}
}

// HashPassword hashes a password using bcrypt.
func (h *PasswordHasher) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(bytes), nil
}

// VerifyPassword verifies a password against a bcrypt hash.
func (h *PasswordHasher) VerifyPassword(password, hash string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return ErrInvalidPassword
	}
	return nil
}

// KeyEncryption handles encryption/decryption of sensitive keys.
type KeyEncryption struct {
	masterKey []byte
}

// NewKeyEncryption creates a new key encryption handler.
func NewKeyEncryption(masterKey string) (*KeyEncryption, error) {
	// Derive 32-byte key from master key using SHA-256
	hash := sha256.Sum256([]byte(masterKey))
	return &KeyEncryption{masterKey: hash[:]}, nil
}

// Encrypt encrypts data using AES-256-GCM.
func (ke *KeyEncryption) Encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(ke.masterKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts data using AES-256-GCM.
func (ke *KeyEncryption) Decrypt(ciphertext string) ([]byte, error) {
	data, err := base64.URLEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(ke.masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// LoadPrivateKeyFromPEM loads an RSA private key from PEM data.
func LoadPrivateKeyFromPEM(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8
		keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		var ok bool
		key, ok = keyInterface.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not RSA")
		}
	}

	return key, nil
}

// LoadPublicKeyFromPEM loads an RSA public key from PEM data.
func LoadPublicKeyFromPEM(pemData []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	keyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	key, ok := keyInterface.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not RSA")
	}

	return key, nil
}

// GenerateRSAKeyPair generates a new RSA key pair.
func GenerateRSAKeyPair(bits int) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}
	return privateKey, &privateKey.PublicKey, nil
}

// PrivateKeyToPEM converts an RSA private key to PEM format.
func PrivateKeyToPEM(key *rsa.PrivateKey) []byte {
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}
	return pem.EncodeToMemory(block)
}

// PublicKeyToPEM converts an RSA public key to PEM format.
func PublicKeyToPEM(key *rsa.PublicKey) ([]byte, error) {
	keyBytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}
	block := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: keyBytes,
	}
	return pem.EncodeToMemory(block), nil
}

// UserIDFromContext extracts user ID from context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value("user_id").(string)
	return userID, ok
}

// WithUserID adds user ID to context.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, "user_id", userID)
}

// InstitutionIDFromContext extracts institution ID from context.
func InstitutionIDFromContext(ctx context.Context) (string, bool) {
	instID, ok := ctx.Value("institution_id").(string)
	return instID, ok
}

// WithInstitutionID adds institution ID to context.
func WithInstitutionID(ctx context.Context, institutionID string) context.Context {
	return context.WithValue(ctx, "institution_id", institutionID)
}

