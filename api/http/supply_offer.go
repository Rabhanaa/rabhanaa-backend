package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"rabhana/auction/model"
	"rabhana/auction/service"
)

type SupplyOfferHandler struct {
	service *service.SupplyOfferingService
}

func NewSupplyOfferHandler(service *service.SupplyOfferingService) *SupplyOfferHandler {
	return &SupplyOfferHandler{service: service}
}

func (h *SupplyOfferHandler) PlaceOffer(c *gin.Context) {
	requestID := c.Param("id")
	uid, err := uuid.Parse(requestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}

	var req model.PlaceSupplyOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := getUserID(c)

	if err := h.service.PlaceOffer(c.Request.Context(), userID, uid, req); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "offer placed successfully"})
}

func (h *SupplyOfferHandler) ListMyOffers(c *gin.Context) {
	userID := getUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	offers, total, err := h.service.ListMyOffers(c.Request.Context(), userID, int32(page), int32(pageSize))
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"offers": offers,
		"total":  total,
	})
}

func (h *SupplyOfferHandler) ListByRequest(c *gin.Context) {
	requestID := c.Param("id")
	uid, err := uuid.Parse(requestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request id"})
		return
	}

	userID := getUserID(c)

	offers, err := h.service.ListOffersByRequest(c.Request.Context(), uid, userID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"offers": offers,
	})
}
