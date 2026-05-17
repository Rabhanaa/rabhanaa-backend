package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"rabhana/auction/model"
	"rabhana/auction/service"
)

type SellAuctionHandler struct {
	service *service.SellAuctionService
}

func NewSellAuctionHandler(service *service.SellAuctionService) *SellAuctionHandler {
	return &SellAuctionHandler{service: service}
}

func (h *SellAuctionHandler) Create(c *gin.Context) {
	var req model.CreateSellAuctionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := getUserID(c)

	resp, err := h.service.CreateSellAuction(c.Request.Context(), userID, req, nil, "")
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *SellAuctionHandler) ListMine(c *gin.Context) {
	userID := getUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	auctions, total, err := h.service.ListMySellAuctions(c.Request.Context(), userID, int32(page), int32(pageSize))
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"auctions": auctions,
		"total":    total,
	})
}

func (h *SellAuctionHandler) List(c *gin.Context) {
	userID := getUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// Parse optional interest filter
	var interestIDs []int32
	if interestParam := c.Query("interest_id"); interestParam != "" {
		if id, err := strconv.Atoi(interestParam); err == nil {
			interestIDs = append(interestIDs, int32(id))
		}
	}

	auctions, total, err := h.service.ListActiveAuctions(c.Request.Context(), userID, int32(page), int32(pageSize), interestIDs)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"auctions": auctions,
		"total":    total,
	})
}

func (h *SellAuctionHandler) Search(c *gin.Context) {
	userID := getUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	searchTerm := c.Query("q")

	if searchTerm == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "search query is required"})
		return
	}

	// Parse optional interest filter
	var interestIDs []int32
	if interestParam := c.Query("interest_id"); interestParam != "" {
		if id, err := strconv.Atoi(interestParam); err == nil {
			interestIDs = append(interestIDs, int32(id))
		}
	}

	auctions, total, err := h.service.SearchAuctions(c.Request.Context(), userID, searchTerm, int32(page), int32(pageSize), interestIDs)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"auctions": auctions,
		"total":    total,
	})
}

func (h *SellAuctionHandler) GetDetail(c *gin.Context) {
	auctionID := c.Param("id")
	uid, err := uuid.Parse(auctionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid auction id"})
		return
	}

	userID := getUserID(c)

	auction, err := h.service.GetSellAuctionDetail(c.Request.Context(), uid, userID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, auction)
}

func (h *SellAuctionHandler) Cancel(c *gin.Context) {
	auctionID := c.Param("id")
	uid, err := uuid.Parse(auctionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid auction id"})
		return
	}

	userID := getUserID(c)

	err = h.service.CancelSellAuction(c.Request.Context(), userID, uid)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "auction cancelled"})
}

func getUserID(c *gin.Context) int32 {
	return int32(c.GetInt("userID"))
}
