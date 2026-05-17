package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"rabhana/auction/service"
)

type SellSelectionHandler struct {
	service *service.SellSelectionService
}

func NewSellSelectionHandler(service *service.SellSelectionService) *SellSelectionHandler {
	return &SellSelectionHandler{service: service}
}

func (h *SellSelectionHandler) SelectWinner(c *gin.Context) {
	auctionID := c.Param("id")
	auctionUID, err := uuid.Parse(auctionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid auction id"})
		return
	}

	var req struct {
		BidPublicID string `json:"bid_public_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bidUID, err := uuid.Parse(req.BidPublicID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bid id"})
		return
	}

	userID := getUserID(c)

	if err := h.service.SelectWinner(c.Request.Context(), userID, auctionUID, bidUID); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "winner selected successfully"})
}
