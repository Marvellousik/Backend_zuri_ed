package auth_test

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"zuri/shared/pkg/auth"
)

func TestRS256Token_GenerateAndValidate(t *testing.T) {
	// Generate RSA 2048 key pair
	privKey, pubKey, err := auth.GenerateRSAKeyPair(2048)
	require.NoError(t, err)
	require.NotNil(t, privKey)
	require.NotNil(t, pubKey)

	jwtManager := auth.NewJWTManager(privKey, pubKey)
	require.NotNil(t, jwtManager)

	userID := "usr_abc12345"
	email := "student@zuri.edu"
	role := "student"
	accessTTL := 15 * time.Minute
	refreshTTL := 24 * time.Hour

	tokenPair, err := jwtManager.GenerateTokenPair(userID, email, role, accessTTL, refreshTTL)
	require.NoError(t, err)
	require.NotNil(t, tokenPair)

	assert.NotEmpty(t, tokenPair.AccessToken)
	assert.NotEmpty(t, tokenPair.RefreshToken)
	assert.Equal(t, "Bearer", tokenPair.TokenType)
	assert.True(t, tokenPair.ExpiresAt.After(time.Now()))

	// Validate with full JWTManager
	claims, err := jwtManager.ValidateToken(tokenPair.AccessToken)
	require.NoError(t, err)
	require.NotNil(t, claims)

	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, role, claims.Role)
	assert.Equal(t, "access", claims.TokenType)
	assert.Equal(t, userID, claims.Subject)
	assert.NotEmpty(t, claims.ID)

	// Validate with public-key-only JWTManager (API Gateway use case)
	pubOnlyManager := auth.NewJWTManagerWithPublicKeyOnly(pubKey)
	claimsFromPub, err := pubOnlyManager.ValidateToken(tokenPair.AccessToken)
	require.NoError(t, err)
	require.NotNil(t, claimsFromPub)
	assert.Equal(t, userID, claimsFromPub.UserID)
	assert.Equal(t, email, claimsFromPub.Email)
	assert.Equal(t, role, claimsFromPub.Role)

	// Refresh token validation helper (HashRefreshToken)
	refreshTokenHash := auth.HashRefreshToken(tokenPair.RefreshToken)
	assert.NotEmpty(t, refreshTokenHash)
	assert.Equal(t, refreshTokenHash, auth.HashRefreshToken(tokenPair.RefreshToken), "HashRefreshToken must be deterministic")
}

func TestPasswordHasher_BcryptCost12(t *testing.T) {
	cost := 12
	hasher := auth.NewPasswordHasher(cost)
	require.NotNil(t, hasher)

	password := "Correct-Horse-Battery-Staple#2026"

	hashed, err := hasher.HashPassword(password)
	require.NoError(t, err)
	assert.NotEmpty(t, hashed)

	// Invariant: bcrypt hash cost must equal 12
	derivedCost, err := bcrypt.Cost([]byte(hashed))
	require.NoError(t, err)
	assert.Equal(t, 12, derivedCost, "Password hasher must use bcrypt cost 12")

	// Invariant: verification succeeds for correct password
	err = hasher.VerifyPassword(password, hashed)
	assert.NoError(t, err)

	// Invariant: verification fails for incorrect password
	err = hasher.VerifyPassword("Incorrect-Password", hashed)
	assert.ErrorIs(t, err, auth.ErrInvalidPassword)

	// Invariant: verification fails for empty password
	err = hasher.VerifyPassword("", hashed)
	assert.ErrorIs(t, err, auth.ErrInvalidPassword)
}

func TestAES256GCM_KeyEncryption(t *testing.T) {
	masterKey := "super-secure-aes256-master-key-32b"
	ke, err := auth.NewKeyEncryption(masterKey)
	require.NoError(t, err)
	require.NotNil(t, ke)

	// Invariant: roundtrip arbitrary sensitive data
	plaintext := []byte("confidential-jwt-signing-secret-key-data")
	ciphertext, err := ke.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)
	assert.NotEqual(t, string(plaintext), ciphertext)

	decrypted, err := ke.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)

	// Invariant: non-deterministic ciphertext (unique nonce per encryption)
	ciphertext2, err := ke.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, ciphertext, ciphertext2, "AES-GCM must use random nonce for distinct ciphertexts")

	decrypted2, err := ke.Decrypt(ciphertext2)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted2)

	// Invariant: Private key PEM encryption/decryption roundtrip
	privKey, pubKey, err := auth.GenerateRSAKeyPair(2048)
	require.NoError(t, err)

	privPEM := auth.PrivateKeyToPEM(privKey)
	encryptedPrivPEM, err := ke.Encrypt(privPEM)
	require.NoError(t, err)

	decryptedPrivPEM, err := ke.Decrypt(encryptedPrivPEM)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(privPEM, decryptedPrivPEM))

	loadedPrivKey, err := auth.LoadPrivateKeyFromPEM(decryptedPrivPEM)
	require.NoError(t, err)
	assert.Equal(t, privKey.N, loadedPrivKey.N)
	assert.Equal(t, privKey.D, loadedPrivKey.D)

	// Public key PEM roundtrip
	pubPEM, err := auth.PublicKeyToPEM(pubKey)
	require.NoError(t, err)
	loadedPubKey, err := auth.LoadPublicKeyFromPEM(pubPEM)
	require.NoError(t, err)
	assert.Equal(t, pubKey.N, loadedPubKey.N)
	assert.Equal(t, pubKey.E, loadedPubKey.E)

	// Invariant: Decrypt invalid ciphertext errors
	t.Run("ciphertext too short", func(t *testing.T) {
		shortCiphertext := base64.URLEncoding.EncodeToString([]byte("short"))
		_, err := ke.Decrypt(shortCiphertext)
		assert.Error(t, err)
	})

	t.Run("invalid base64", func(t *testing.T) {
		_, err := ke.Decrypt("!!!not-base-64!!!")
		assert.Error(t, err)
	})

	t.Run("tampered ciphertext authentication failure", func(t *testing.T) {
		rawBytes, err := base64.URLEncoding.DecodeString(ciphertext)
		require.NoError(t, err)

		// Tamper with ciphertext byte (violates GCM auth tag)
		rawBytes[len(rawBytes)-1] ^= 0xFF
		tamperedCiphertext := base64.URLEncoding.EncodeToString(rawBytes)

		_, err = ke.Decrypt(tamperedCiphertext)
		assert.Error(t, err, "Decryption must fail when authentication tag is tampered")
	})

	t.Run("wrong master key cannot decrypt", func(t *testing.T) {
		wrongKE, err := auth.NewKeyEncryption("different-master-key-32-chars-long")
		require.NoError(t, err)

		_, err = wrongKE.Decrypt(ciphertext)
		assert.Error(t, err, "Decryption with different master key must fail")
	})
}

func TestRS256Token_Rejection(t *testing.T) {
	privKey, pubKey, err := auth.GenerateRSAKeyPair(2048)
	require.NoError(t, err)
	manager := auth.NewJWTManager(privKey, pubKey)

	t.Run("rejects expired token", func(t *testing.T) {
		// Create token pair with negative TTL (already expired)
		tokenPair, err := manager.GenerateTokenPair("user-1", "user1@example.com", "student", -5*time.Minute, 1*time.Hour)
		require.NoError(t, err)

		claims, err := manager.ValidateToken(tokenPair.AccessToken)
		assert.Nil(t, claims)
		assert.ErrorIs(t, err, auth.ErrTokenExpired, "Expected ErrTokenExpired for expired access token")
	})

	t.Run("rejects token with invalid signature from another key", func(t *testing.T) {
		// Generate second unrelated key pair
		foreignPrivKey, foreignPubKey, err := auth.GenerateRSAKeyPair(2048)
		require.NoError(t, err)
		foreignManager := auth.NewJWTManager(foreignPrivKey, foreignPubKey)

		foreignTokenPair, err := foreignManager.GenerateTokenPair("user-2", "user2@example.com", "admin", 15*time.Minute, 1*time.Hour)
		require.NoError(t, err)

		// Validate token from foreign manager against original manager
		claims, err := manager.ValidateToken(foreignTokenPair.AccessToken)
		assert.Nil(t, claims)
		assert.ErrorIs(t, err, auth.ErrInvalidToken, "Expected ErrInvalidToken for mismatched RSA signature")
	})

	t.Run("rejects tampered token string", func(t *testing.T) {
		tokenPair, err := manager.GenerateTokenPair("user-3", "user3@example.com", "student", 15*time.Minute, 1*time.Hour)
		require.NoError(t, err)

		parts := strings.Split(tokenPair.AccessToken, ".")
		require.Len(t, parts, 3)

		// Tamper with payload
		tamperedToken := parts[0] + ".e30." + parts[2]
		claims, err := manager.ValidateToken(tamperedToken)
		assert.Nil(t, claims)
		assert.ErrorIs(t, err, auth.ErrInvalidToken)

		// Tamper with signature
		tamperedSig := parts[0] + "." + parts[1] + ".AAAA"
		claims, err = manager.ValidateToken(tamperedSig)
		assert.Nil(t, claims)
		assert.ErrorIs(t, err, auth.ErrInvalidToken)
	})

	t.Run("rejects token with invalid token_type claim", func(t *testing.T) {
		// Manually create an RS256 token signed by privKey, but with token_type="refresh"
		now := time.Now()
		refreshClaims := auth.Claims{
			UserID:    "user-4",
			Email:     "user4@example.com",
			Role:      "student",
			TokenType: "refresh", // Invalid for access token validation
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
				IssuedAt:  jwt.NewNumericDate(now),
				NotBefore: jwt.NewNumericDate(now),
				Subject:   "user-4",
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, refreshClaims)
		tokenString, err := token.SignedString(privKey)
		require.NoError(t, err)

		claims, err := manager.ValidateToken(tokenString)
		assert.Nil(t, claims)
		assert.ErrorIs(t, err, auth.ErrInvalidToken, "Expected ErrInvalidToken when token_type != 'access'")
	})

	t.Run("rejects malformed token strings", func(t *testing.T) {
		malformedTokens := []string{
			"",
			"not-a-token",
			"header.payload",
			"header.payload.signature.extra",
		}

		for _, mt := range malformedTokens {
			claims, err := manager.ValidateToken(mt)
			assert.Nil(t, claims)
			assert.ErrorIs(t, err, auth.ErrInvalidToken)
		}
	})

	t.Run("rejects token with wrong signing method alg", func(t *testing.T) {
		// Sign token with HMAC HS256 instead of RS256
		now := time.Now()
		claims := auth.Claims{
			UserID:    "user-5",
			Email:     "user5@example.com",
			Role:      "student",
			TokenType: "access",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
				IssuedAt:  jwt.NewNumericDate(now),
				NotBefore: jwt.NewNumericDate(now),
			},
		}

		hsToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := hsToken.SignedString([]byte("secret-key"))
		require.NoError(t, err)

		validatedClaims, err := manager.ValidateToken(tokenString)
		assert.Nil(t, validatedClaims)
		assert.ErrorIs(t, err, auth.ErrInvalidToken)
	})
}

func TestVerificationCode_LengthAndEntropy(t *testing.T) {
	codes := make(map[string]bool)
	for i := 0; i < 50; i++ {
		code := auth.GenerateVerificationCode()
		assert.Len(t, code, 6, "Verification code must be 6 digits")
		codes[code] = true
	}
	// With 50 random 6-digit codes, we expect good entropy (at least 45 unique codes)
	assert.GreaterOrEqual(t, len(codes), 45, "Verification codes should have sufficient randomness")
}

func TestTenantRS256Token_InstitutionScoping(t *testing.T) {
	privKey, pubKey, err := auth.GenerateRSAKeyPair(2048)
	require.NoError(t, err)

	jwtManager := auth.NewJWTManager(privKey, pubKey)
	userID := "usr_student_01"
	email := "student@veritas.edu"
	role := "student"
	institutionID := "inst_veritas_abuja"

	tokenPair, err := jwtManager.GenerateTenantTokenPair(userID, email, role, institutionID, 15*time.Minute, 24*time.Hour)
	require.NoError(t, err)
	require.NotNil(t, tokenPair)

	claims, err := jwtManager.ValidateToken(tokenPair.AccessToken)
	require.NoError(t, err)
	require.NotNil(t, claims)

	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, role, claims.Role)
	assert.Equal(t, institutionID, claims.InstitutionID)
}
