package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"rabhana/order/service"
)

type OrderHandler struct {
	service *service.OrderService
}

func NewOrderHandler(service *service.OrderService) *OrderHandler {
	return &OrderHandler{service: service}
}

func (h *OrderHandler) List(c *gin.Context) {
	userID := getUserID(c)

	orders, total, err := h.service.ListMyOrders(c.Request.Context(), userID, 1, 20)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": orders,
		"total":  total,
	})
}

func (h *OrderHandler) GetDetail(c *gin.Context) {
	orderID := c.Param("id")
	uid, err := uuid.Parse(orderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	userID := getUserID(c)

	order, err := h.service.GetOrderDetail(c.Request.Context(), userID, uid)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, order)
}

func (h *OrderHandler) Confirm(c *gin.Context) {
	orderID := c.Param("id")
	uid, err := uuid.Parse(orderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	userID := getUserID(c)

	err = h.service.ConfirmOrder(c.Request.Context(), userID, uid)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "order confirmed"})
}
