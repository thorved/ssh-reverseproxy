package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/thorved/ssh-reverseproxy/backend/internal/auth"
	"github.com/thorved/ssh-reverseproxy/backend/internal/models"
)

const userContextKey = "currentUser"

func Session(authService *auth.Service, cookieName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(cookieName)
		if err == nil && token != "" {
			user, lookupErr := authService.GetUserBySession(c.Request.Context(), token)
			if lookupErr == nil {
				c.Set(userContextKey, user)
			}
		}
		c.Next()
	}
}

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok || !user.IsActive {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		c.Next()
	}
}

func RequireRole(role models.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		if user.Role != role {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Next()
	}
}

func CurrentUser(c *gin.Context) (*models.User, bool) {
	value, ok := c.Get(userContextKey)
	if !ok {
		return nil, false
	}
	user, ok := value.(*models.User)
	return user, ok
}
