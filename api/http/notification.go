package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"rabhana/notification/service"
)

type RegisterDeviceTokenRequest struct {
	Token    string `json:"token" binding:"required"`
	Platform string `json:"platform" binding:"required"`
}

type DeregisterDeviceTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

type NotificationHandler struct {
	service *service.NotificationService
}

func NewNotificationHandler(service *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: service}
}

func (h *NotificationHandler) List(c *gin.Context) {
	userID := getUserID(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	notifications, err := h.service.ListNotifications(c.Request.Context(), userID, int32(limit))
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"notifications": notifications})
}

func (h *NotificationHandler) MarkAsRead(c *gin.Context) {
	notificationID, _ := strconv.Atoi(c.Param("id"))
	userID := getUserID(c)

	if err := h.service.MarkAsRead(c.Request.Context(), userID, int32(notificationID)); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "marked as read"})
}

func (h *NotificationHandler) MarkAllAsRead(c *gin.Context) {
	userID := getUserID(c)

	if err := h.service.MarkAllAsRead(c.Request.Context(), userID); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "all marked as read"})
}

func (h *NotificationHandler) RegisterDeviceToken(c *gin.Context) {
	var req RegisterDeviceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Platform != "web" && req.Platform != "mobile" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform must be 'web' or 'mobile'"})
		return
	}

	userID := getUserID(c)
	if err := h.service.RegisterDeviceToken(c.Request.Context(), userID, req.Token, req.Platform); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "تم التسجيل"})
}

func (h *NotificationHandler) DeregisterDeviceToken(c *gin.Context) {
	var req DeregisterDeviceTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := h.service.DeregisterDeviceToken(c.Request.Context(), req.Token); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "تم الإلغاء"})
}
