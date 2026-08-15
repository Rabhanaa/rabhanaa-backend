package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	authSvc "rabhana/auth/service"
	"rabhana/pkg/errs"
)

func Auth(authService *authSvc.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   errs.ErrUnauthorized.Error(),
				"message": errs.GetArabicMessage(errs.ErrUnauthorized),
			})
			return
		}

		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   errs.ErrUnauthorized.Error(),
				"message": errs.GetArabicMessage(errs.ErrUnauthorized),
			})
			return
		}

		claims, err := authService.ValidateToken(tokenParts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   errs.ErrUnauthorized.Error(),
				"message": errs.GetArabicMessage(errs.ErrUnauthorized),
			})
			return
		}

		c.Set("userPublicID", claims.UserID.String())
		c.Set("isAdmin", claims.IsAdmin)

		user, err := authService.GetUserByPublicID(c.Request.Context(), claims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   errs.ErrUnauthorized.Error(),
				"message": errs.GetArabicMessage(errs.ErrUnauthorized),
			})
			return
		}
		// Revocation. Flipping user_sessions.is_current does nothing here — this
		// middleware never reads that table — so a password reset stamps
		// password_changed_at and any token minted before it is stale. Without
		// this a stolen token would outlive a reset by the full 365-day TTL.
		if user.PasswordChangedAt.Valid && claims.IssuedAt != nil &&
			claims.IssuedAt.Time.Before(user.PasswordChangedAt.Time) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   errs.ErrUnauthorized.Error(),
				"message": errs.GetArabicMessage(errs.ErrUnauthorized),
			})
			return
		}

		c.Set("userID", int(user.ID))

		c.Next()
	}
}

func Admin() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, exists := c.Get("isAdmin")
		if !exists || !isAdmin.(bool) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   errs.ErrAdminOnly.Error(),
				"message": errs.GetArabicMessage(errs.ErrAdminOnly),
			})
			return
		}
		c.Next()
	}
}
