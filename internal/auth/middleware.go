// Package auth contains the Gin middleware for JWT authentication.
package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// contextKeyUserID and contextKeyUsername are the Gin context keys
// used to pass authenticated user data to downstream handlers.
const (
	ContextKeyUserID   = "userID"
	ContextKeyUsername = "username"
)

// Middleware returns a Gin handler that validates the JWT.
//
// Token lookup order:
//  1. ?token=<jwt> query parameter   — used by WebSocket clients, because
//     browsers cannot send custom headers during the WS handshake.
//  2. Authorization: Bearer <jwt>    — used by regular REST API calls.
//
// On success it injects userID and username into the Gin context.
// On failure it aborts with 401 Unauthorized.
func Middleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string

		// 1. Try the query parameter first (WebSocket upgrade requests).
		if t := c.Query("token"); t != "" {
			tokenString = t
		} else {
			// 2. Fall back to Authorization header for REST calls.
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization required: provide Bearer token or ?token= query param"})
				return
			}

			// Strip the "Bearer " prefix — be defensive about casing.
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header format must be: Bearer <token>"})
				return
			}
			tokenString = parts[1]
		}

		claims, err := ValidateToken(tokenString, jwtSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		// Store the validated claims in context so handlers don't need to
		// re-parse the token themselves.
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)
		c.Next()
	}
}
