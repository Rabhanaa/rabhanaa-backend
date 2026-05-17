package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"rabhana/db/sqlc"
	"rabhana/pkg/errs"
)

// UserContext holds user + subscription info
type UserContext struct {
	UserID                   int32
	UserStatus               string
	TierName                 *string
	CanCreateSellAuctions    bool
	CanCreateBuyRequests     bool
	CanPlaceBids             bool
	CanMakeOffers            bool
	MaxSellAuctionsPerMonth  *int32
	MaxBuyRequestsPerMonth   *int32
	MaxBidsPerMonth          *int32
	MaxOffersPerMonth        *int32
	AuctionsCreatedThisMonth int32
	RequestsCreatedThisMonth int32
	BidsPlacedThisMonth      int32
	OffersMadeThisMonth      int32
	SubscriptionID           *int32
	IsUnlimited              bool
}

func AccountStatus(queries *sqlc.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := c.Get("userPublicID")
		if !exists {
			c.Next()
			return
		}

		userID := getUserIDFromContext(c)
		if userID == 0 {
			c.Next()
			return
		}

		row, err := queries.GetUserWithSubscription(c.Request.Context(), userID)
		if err != nil {
			if err == pgx.ErrNoRows {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"error":   errs.ErrUnauthorized.Error(),
					"message": errs.GetArabicMessage(errs.ErrUnauthorized),
				})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error":   "INTERNAL_ERROR",
				"message": "خطأ في النظام",
			})
			return
		}

		// Enforce ban/suspend for non-admin users. Admins bypass so they can
		// reach unsuspend/unban endpoints even if their own account were somehow affected.
		isAdmin, _ := c.Get("isAdmin")
		if isAdmin != true {
			switch row.UserStatus {
			case "banned":
				reason := ""
				if row.UserBannedReason.Valid {
					reason = row.UserBannedReason.String
				}
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error":   errs.ErrUserBanned.Error(),
					"message": errs.GetArabicMessage(errs.ErrUserBanned),
					"reason":  reason,
				})
				return
			case "suspended":
				if row.UserSuspendedUntil.Valid && row.UserSuspendedUntil.Time.Before(time.Now()) {
					// Lazy restore: suspension expired, flip to active.
					_, _ = queries.LazyRestoreExpiredSuspension(c.Request.Context(), userID)
					row.UserStatus = "active"
				} else {
					reason := ""
					if row.UserSuspensionReason.Valid {
						reason = row.UserSuspensionReason.String
					}
					c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
						"error":   errs.ErrUserSuspended.Error(),
						"message": errs.GetArabicMessage(errs.ErrUserSuspended),
						"reason":  reason,
					})
					return
				}
			}
		}

		userCtx := &UserContext{
			UserID:     row.UserID,
			UserStatus: row.UserStatus,
		}

		if row.TierName.Valid {
			tierName := row.TierName.String
			userCtx.TierName = &tierName
			userCtx.CanCreateSellAuctions = row.CanCreateSellAuctions.Valid && row.CanCreateSellAuctions.Bool
			userCtx.CanCreateBuyRequests = row.CanCreateBuyRequests.Valid && row.CanCreateBuyRequests.Bool
			userCtx.CanPlaceBids = row.CanPlaceBids.Valid && row.CanPlaceBids.Bool
			userCtx.CanMakeOffers = row.CanMakeOffers.Valid && row.CanMakeOffers.Bool
			userCtx.SubscriptionID = &row.SubscriptionID.Int32
			userCtx.AuctionsCreatedThisMonth = row.AuctionsCreatedThisMonth.Int32
			userCtx.RequestsCreatedThisMonth = row.RequestsCreatedThisMonth.Int32
			userCtx.BidsPlacedThisMonth = row.BidsPlacedThisMonth.Int32
			userCtx.OffersMadeThisMonth = row.OffersMadeThisMonth.Int32

			if row.MaxSellAuctionsPerMonth.Valid {
				maxAuctions := row.MaxSellAuctionsPerMonth.Int32
				userCtx.MaxSellAuctionsPerMonth = &maxAuctions
			} else {
				userCtx.IsUnlimited = true
			}

			if row.MaxBuyRequestsPerMonth.Valid {
				maxRequests := row.MaxBuyRequestsPerMonth.Int32
				userCtx.MaxBuyRequestsPerMonth = &maxRequests
			}

			if row.MaxBidsPerMonth.Valid {
				maxBids := row.MaxBidsPerMonth.Int32
				userCtx.MaxBidsPerMonth = &maxBids
			}

			if row.MaxOffersPerMonth.Valid {
				maxOffers := row.MaxOffersPerMonth.Int32
				userCtx.MaxOffersPerMonth = &maxOffers
			}
		}

		c.Set("userContext", userCtx)
		c.Set("userTier", userCtx.TierName)
		c.Next()
	}
}

func getUserIDFromContext(c *gin.Context) int32 {
	id, exists := c.Get("userID")
	if !exists {
		return 0
	}
	return int32(id.(int))
}

// GetUserContext retrieves the user context from gin context
func GetUserContext(c *gin.Context) *UserContext {
	ctx, exists := c.Get("userContext")
	if !exists {
		return nil
	}
	return ctx.(*UserContext)
}

// GetUserTier retrieves the user's tier name from gin context
func GetUserTier(c *gin.Context) *string {
	tier, exists := c.Get("userTier")
	if !exists {
		return nil
	}
	tierStr := tier.(string)
	return &tierStr
}
