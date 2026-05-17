package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"rabhana/auction/repository"
)

// UserBidsHandler handles user-specific bid endpoints
type UserBidsHandler struct {
	sellBidRepo     repository.SellBidRepository
	supplyOfferRepo repository.SupplyOfferRepository
}

// NewUserBidsHandler creates a new user bids handler
func NewUserBidsHandler(sellBidRepo repository.SellBidRepository, supplyOfferRepo repository.SupplyOfferRepository) *UserBidsHandler {
	return &UserBidsHandler{
		sellBidRepo:     sellBidRepo,
		supplyOfferRepo: supplyOfferRepo,
	}
}

// GetActiveBidCount returns the user's active bid count across both auction types
func (h *UserBidsHandler) GetActiveBidCount(c *gin.Context) {
	userID := getUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Count active sell bids
	sellBidCount, err := h.sellBidRepo.CountActiveBidsByBidder(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count sell bids"})
		return
	}

	// Count active supply offers
	supplyOfferCount, err := h.supplyOfferRepo.CountActiveOffersBySupplier(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count supply offers"})
		return
	}

	totalCount := sellBidCount + supplyOfferCount

	c.JSON(http.StatusOK, gin.H{
		"active_bid_count": totalCount,
		"max_active_bids":  3,
		"sell_bids":        sellBidCount,
		"supply_offers":    supplyOfferCount,
	})
}
