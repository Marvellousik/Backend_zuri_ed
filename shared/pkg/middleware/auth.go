// Package middleware provides HTTP middleware for authentication and logging.
package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// AuthRequired returns a Gin middleware that validates requests.
//
// Primary path (gateway-forwarded): trusts X-User-ID + X-Internal-Key headers
// that the gateway injects after validating the JWT. This is the path taken
// for all WebSocket and HTTP requests that arrive via the gateway proxy.
//
// Fallback path (direct calls): parses the raw JWT from the Authorization
// header or ?token= query param without re-validating the signature (the
// gateway is the single validation point).
func AuthRequired() gin.HandlerFunc {
	// Read the internal API key once at middleware creation time.
	// Falls back to the same default the gateway uses.
	internalKey := os.Getenv("INTERNAL_API_KEY")
	if internalKey == "" {
		internalKey = "dev-internal-key"
	}

	return func(c *gin.Context) {
		// ── Primary path: gateway pre-validated request ──────────────────────
		// The gateway strips the JWT and injects X-User-ID + X-Internal-Key
		// after validating the token. Trust this if the internal key matches.
		if userID := c.GetHeader("X-User-ID"); userID != "" {
			if c.GetHeader("X-Internal-Key") == internalKey {
				c.Set("user_id", userID)
				c.Next()
				return
			}
			// X-User-ID present but key mismatch — reject rather than fall through
			// to prevent header spoofing from external clients.
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid internal key"})
			c.Abort()
			return
		}

		// Reject any request that does not come from the trusted Gateway with a valid internal key
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized: trusted internal gateway authentication required"})
		c.Abort()
		return
	}
}
