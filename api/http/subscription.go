package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"rabhana/db/sqlc"
)

type SubscriptionHandler struct {
	queries *sqlc.Queries
}

func NewSubscriptionHandler(queries *sqlc.Queries) *SubscriptionHandler {
	return &SubscriptionHandler{queries: queries}
}

func (h *SubscriptionHandler) GetMySubscription(c *gin.Context) {
	userID := getUserID(c)

	userSub, err := h.queries.GetUserWithSubscription(c.Request.Context(), userID)
	if err != nil {
		// Return free tier if not found
		c.JSON(http.StatusOK, gin.H{
			"tier":      "free",
			"is_active": true,
		})
		return
	}

	c.JSON(http.StatusOK, userSub)
}

func (h *SubscriptionHandler) GetMyReferralCode(c *gin.Context) {
	userID := getUserID(c)

	code, err := h.queries.GetReferralCodeByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no referral code found"})
		return
	}

	c.JSON(http.StatusOK, code)
}

func (h *SubscriptionHandler) ApplyReferralCode(c *gin.Context) {
	_ = getUserID(c) // TODO: use userID in actual implementation

	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Simplified - would need actual referral logic
	c.JSON(http.StatusOK, gin.H{"message": "referral code applied"})
}

func (h *SubscriptionHandler) GetSubscriptionTiers(c *gin.Context) {
	tiers, err := h.queries.GetSubscriptionTiers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tiers": tiers})
}

func (h *SubscriptionHandler) GetSubscriptionStatus(c *gin.Context) {
	userID := getUserID(c)

	sub, err := h.queries.GetUserSubscriptionStatus(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"tier_name": "free",
			"is_active": true,
		})
		return
	}

	c.JSON(http.StatusOK, sub)
}
