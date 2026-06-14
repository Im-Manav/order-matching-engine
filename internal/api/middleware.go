package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/Im-Manav/ome/internal/ports"
	"github.com/Im-Manav/ome/internal/service"
	"github.com/Im-Manav/ome/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// RateLimit — max orders per user per minute
	RateLimitRequests = 60
	ContextUserID     = "userID"
	ContextUserEmail  = "userEmail"
	ContextJTI        = "jti"
)

// Auth is the JWT authentication middleware.
// It validates the token, checks the blocklist, and injects
// the userID into the Gin context for downstream handlers.
func Auth(authSvc *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authorization header required",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid authorization format, expected: Bearer <token>",
			})
			return
		}

		tokenString := parts[1]

		claims, err := authSvc.ValidateToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired token",
			})
			return
		}

		// Check JWT blocklist — token may have been invalidated by logout
		blocked, err := authSvc.IsBlocklisted(c.Request.Context(), claims.JTI)
		if err != nil || blocked {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "token has been revoked",
			})
			return
		}

		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid user id in token",
			})
			return
		}

		c.Set(ContextUserID, userID)
		c.Set(ContextUserEmail, claims.Email)
		c.Set(ContextJTI, claims.JTI)
		c.Next()
	}
}

// RateLimit enforces a per-user request rate limit using Redis.
// Uses a fixed window counter — simple and effective for order APIs.
func RateLimit(cache ports.Cache) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get(ContextUserID)
		if !exists {
			c.Next()
			return
		}

		count, err := cache.IncrWithExpiry(
			c.Request.Context(),
			userID.(uuid.UUID).String(),
			60*time.Second, // 60 second window
		)
		if err != nil {
			// If Redis is down, fail open — don't block legitimate orders
			c.Next()
			return
		}

		if count > RateLimitRequests {
			appErr := errors.ToHTTP(errors.ErrRateLimitExceeded)
			c.AbortWithStatusJSON(appErr.Code, gin.H{
				"error": appErr.Message,
			})
			return
		}
		c.Next()
	}
}
