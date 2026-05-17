package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"rabhana/auction/model"
	"rabhana/auction/service"
)

type BuyRequestHandler struct {
	service *service.BuyRequestService
}

func NewBuyRequestHandler(service *service.BuyRequestService) *BuyRequestHandler {
	return &BuyRequestHandler{service: service}
}

func (h *BuyRequestHandler) Create(c *gin.Context) {
	var req model.CreateBuyRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := getUserID(c)

	resp, err := h.service.CreateBuyRequest(c.Request.Context(), userID, req, nil, "")
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *BuyRequestHandler) ListMine(c *gin.Context) {
	userID := getUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	requests, total, err := h.service.ListMyBuyRequests(c.Request.Context(), userID, int32(page), int32(pageSize))
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"requests": requests,
		"total":    total,
	})
}

func (h *BuyRequestHandler) List(c *gin.Context) {
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

	requests, total, err := h.service.ListActiveBuyRequests(c.Request.Context(), userID, int32(page), int32(pageSize), interestIDs)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"requests": requests,
		"total":    total,
	})
}

func (h *BuyRequestHandler) Search(c *gin.Context) {
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

	requests, total, err := h.service.SearchBuyRequests(c.Request.Context(), userID, searchTerm, int32(page), int32(pageSize), interestIDs)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"requests": requests,
		"total":    total,
	})
}

func (h *BuyRequestHandler) GetDetail(c *gin.Context) {
	requestID := c.Param("id")
	uid, err := uuid.Parse(requestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}

	userID := getUserID(c)

	request, err := h.service.GetBuyRequestDetail(c.Request.Context(), uid, userID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, request)
}

func (h *BuyRequestHandler) Cancel(c *gin.Context) {
	requestID := c.Param("id")
	uid, err := uuid.Parse(requestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}

	userID := getUserID(c)

	err = h.service.CancelBuyRequest(c.Request.Context(), userID, uid)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "request cancelled"})
}
