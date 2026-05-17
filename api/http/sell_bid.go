package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"rabhana/auction/model"
	"rabhana/auction/service"
)

type SellBidHandler struct {
	service *service.SellBiddingService
}

func NewSellBidHandler(service *service.SellBiddingService) *SellBidHandler {
	return &SellBidHandler{service: service}
}

func (h *SellBidHandler) PlaceBid(c *gin.Context) {
	auctionID := c.Param("id")
	uid, err := uuid.Parse(auctionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid auction id"})
		return
	}

	var req model.PlaceSellBidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := getUserID(c)

	if err := h.service.PlaceBid(c.Request.Context(), userID, uid, req); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "bid placed successfully"})
}

func (h *SellBidHandler) ListMyBids(c *gin.Context) {
	userID := getUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	bids, total, err := h.service.ListMyBids(c.Request.Context(), userID, int32(page), int32(pageSize))
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bids":  bids,
		"total": total,
	})
}

func (h *SellBidHandler) ListByAuction(c *gin.Context) {
	auctionID := c.Param("id")
	uid, err := uuid.Parse(auctionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid auction id"})
		return
	}

	userID := getUserID(c)

	bids, err := h.service.ListBidsByAuction(c.Request.Context(), uid, userID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bids": bids,
	})
}
