package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"rabhana/subscription/service"
)

type AdminSubscriptionHandler struct {
	service *service.AdminService
}

func NewAdminSubscriptionHandler(service *service.AdminService) *AdminSubscriptionHandler {
	return &AdminSubscriptionHandler{service: service}
}

func (h *AdminSubscriptionHandler) ListUserSubscriptions(c *gin.Context) {
	uid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	subs, err := h.service.ListUserSubscriptions(c.Request.Context(), uid)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"subscriptions": subs})
}

func (h *AdminSubscriptionHandler) GrantSubscription(c *gin.Context) {
	uid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req struct {
		TierName  string     `json:"tier_name" binding:"required"`
		StartedAt time.Time  `json:"started_at"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	startedAt := req.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	var expiresAt time.Time
	if req.ExpiresAt != nil {
		expiresAt = *req.ExpiresAt
	} else {
		expiresAt = startedAt.AddDate(0, 1, 0) // default 1 month
	}

	sub, _, err := h.service.GrantSubscription(c.Request.Context(), uid, req.TierName, startedAt, expiresAt)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"subscription": sub})
}

func (h *AdminSubscriptionHandler) UpdateSubscription(c *gin.Context) {
	subID, err := strconv.ParseInt(c.Param("sub_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid subscription id"})
		return
	}

	var req struct {
		StartedAt *time.Time `json:"started_at"`
		ExpiresAt *time.Time `json:"expires_at"`
		IsActive  *bool      `json:"is_active"`
		IsPrimary *bool      `json:"is_primary"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sub, err := h.service.UpdateSubscription(c.Request.Context(), int32(subID), req.StartedAt, req.ExpiresAt, req.IsActive, req.IsPrimary)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"subscription": sub})
}
