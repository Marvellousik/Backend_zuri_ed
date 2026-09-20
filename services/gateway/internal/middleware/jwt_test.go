package middleware_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"zuri/services/gateway/internal/middleware"
)

type mockRedisClient struct {
	existsKeys map[string]bool
}

func (m *mockRedisClient) Exists(ctx context.Context, key string) (bool, error) {
	return m.existsKeys[key], nil
}

func generateTestRSAKey(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return privKey, &privKey.PublicKey
}

func createTestToken(t *testing.T, privKey *rsa.PrivateKey, jti, userID, email, role, tokenType, institutionID string, exp time.Duration) string {
	now := time.Now()
	claims := middleware.Claims{
		UserID:        userID,
		Email:         email,
		Role:          role,
		TokenType:     tokenType,
		InstitutionID: institutionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(exp)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenStr, err := token.SignedString(privKey)
	require.NoError(t, err)
	return tokenStr
}

func TestJWTMiddleware_RoleClaimsAndBlacklist(t *testing.T) {
	privKey, pubKey := generateTestRSAKey(t)

	tests := []struct {
		name                  string
		jti                   string
		role                  string
		institutionID         string
		blacklistedKeys       []string
		expectedStatusCode    int
		expectedRole          string
		expectedTokenID       string
		expectedInstitutionID string
	}{
		{
			name:                  "valid token sets role, token_id, and institution_id",
			jti:                   "jti-valid-123",
			role:                  "instructor",
			institutionID:         "inst-veritas-01",
			blacklistedKeys:       nil,
			expectedStatusCode:    http.StatusOK,
			expectedRole:          "instructor",
			expectedTokenID:       "jti-valid-123",
			expectedInstitutionID: "inst-veritas-01",
		},
		{
			name:                  "token blacklisted via token_blacklist:<jti>",
			jti:                   "jti-revoked-token-blacklist",
			role:                  "student",
			institutionID:         "inst-veritas-01",
			blacklistedKeys:       []string{"token_blacklist:jti-revoked-token-blacklist"},
			expectedStatusCode:    http.StatusUnauthorized,
		},
		{
			name:                  "token blacklisted via blacklist:<jti>",
			jti:                   "jti-revoked-blacklist",
			role:                  "admin",
			institutionID:         "",
			blacklistedKeys:       []string{"blacklist:jti-revoked-blacklist"},
			expectedStatusCode:    http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redisMock := &mockRedisClient{existsKeys: make(map[string]bool)}
			for _, k := range tt.blacklistedKeys {
				redisMock.existsKeys[k] = true
			}

			validator := middleware.NewJWTValidator(pubKey, redisMock)
			mw := middleware.JWTMiddleware(validator, nil)

			tokenStr := createTestToken(t, privKey, tt.jti, "user-abc", "user@example.com", tt.role, "access", tt.institutionID, time.Hour)

			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", "Bearer "+tokenStr)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			var capturedRole, capturedTokenID, capturedInstID string
			handler := mw(func(ctx echo.Context) error {
				capturedRole = ctx.Get("role").(string)
				capturedTokenID = ctx.Get("token_id").(string)
				if inst := ctx.Get("institution_id"); inst != nil {
					capturedInstID = inst.(string)
				}
				return ctx.String(http.StatusOK, "success")
			})

			err := handler(c)
			if tt.expectedStatusCode == http.StatusOK {
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Equal(t, tt.expectedRole, capturedRole)
				assert.Equal(t, tt.expectedTokenID, capturedTokenID)
				assert.Equal(t, tt.expectedInstitutionID, capturedInstID)
			} else {
				require.Error(t, err)
				httpErr, ok := err.(*echo.HTTPError)
				require.True(t, ok)
				assert.Equal(t, tt.expectedStatusCode, httpErr.Code)
			}
		})
	}
}
