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

// OptionalAuth identifies the caller when they present a valid token and lets
// them through either way.
//
// Public browsing (#4) needs the same handlers to serve members and visitors:
// duplicating the listing endpoints would fork the filtering logic added in #7
// and #11 and guarantee the copies drift. With this, a signed-in visitor keeps
// their region filter, retailer filter, own-post exclusion and is_owner flag,
// while an anonymous one falls through with userID 0 and gets the unfiltered
// feed.
//
// A bad or revoked token is treated as no token rather than an error — the page
// is public, so there is nothing to deny.
func OptionalAuth(authService *authSvc.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenParts := strings.Split(c.GetHeader("Authorization"), " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.Next()
			return
		}

		claims, err := authService.ValidateToken(tokenParts[1])
		if err != nil {
			c.Next()
			return
		}

		user, err := authService.GetUserByPublicID(c.Request.Context(), claims.UserID)
		if err != nil {
			c.Next()
			return
		}

		// Same revocation rule as Auth: a token minted before the password
		// changed is stale, and here that simply means anonymous.
		if user.PasswordChangedAt.Valid && claims.IssuedAt != nil &&
			claims.IssuedAt.Time.Before(user.PasswordChangedAt.Time) {
			c.Next()
			return
		}

		c.Set("userPublicID", claims.UserID.String())
		c.Set("isAdmin", claims.IsAdmin)
		c.Set("userID", int(user.ID))
		c.Next()
	}
}
