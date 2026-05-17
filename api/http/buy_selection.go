package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"rabhana/auction/service"
)

type BuySelectionHandler struct {
	service *service.BuySelectionService
}

func NewBuySelectionHandler(service *service.BuySelectionService) *BuySelectionHandler {
	return &BuySelectionHandler{service: service}
}

func (h *BuySelectionHandler) AcceptOffer(c *gin.Context) {
	requestID := c.Param("id")
	requestUID, err := uuid.Parse(requestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}

	var req struct {
		OfferPublicID string `json:"offer_public_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	offerUID, err := uuid.Parse(req.OfferPublicID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid offer id"})
		return
	}

	userID := getUserID(c)

	if err := h.service.AcceptOffer(c.Request.Context(), userID, requestUID, offerUID); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "offer accepted successfully"})
}
