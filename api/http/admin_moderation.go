package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"rabhana/auction/service"
)

type AdminModerationHandler struct {
	moderationService *service.ModerationService
}

func NewAdminModerationHandler(moderationService *service.ModerationService) *AdminModerationHandler {
	return &AdminModerationHandler{moderationService: moderationService}
}

type moderationReasonRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// ListPending returns posts awaiting review; ListPublished returns live and
// suspended ones so an admin can take a post down or restore it.
func (h *AdminModerationHandler) ListPending(c *gin.Context) {
	h.list(c, h.moderationService.ListPending)
}

func (h *AdminModerationHandler) ListPublished(c *gin.Context) {
	h.list(c, h.moderationService.ListPublished)
}

type listFn func(ctx context.Context, page, pageSize int32) ([]service.PendingPost, int64, error)

func (h *AdminModerationHandler) list(c *gin.Context, fn listFn) {
	page, pageSize := paginationParams(c)
	posts, total, err := fn(c.Request.Context(), page, pageSize)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"posts": posts, "total": total, "page": page, "page_size": pageSize,
	})
}

func (h *AdminModerationHandler) Approve(c *gin.Context) {
	h.publish(c, "approved")
}

func (h *AdminModerationHandler) publish(c *gin.Context, resultStatus string) {
	postType, id, ok := h.target(c)
	if !ok {
		return
	}
	if err := h.moderationService.Approve(c.Request.Context(), adminIDFrom(c), postType, id); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": resultStatus})
}

func (h *AdminModerationHandler) Reject(c *gin.Context) {
	postType, id, ok := h.target(c)
	if !ok {
		return
	}
	var req moderationReasonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_BODY", "message": err.Error()})
		return
	}
	if err := h.moderationService.Reject(c.Request.Context(), adminIDFrom(c), postType, id, req.Reason); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "rejected"})
}

func (h *AdminModerationHandler) Suspend(c *gin.Context) {
	postType, id, ok := h.target(c)
	if !ok {
		return
	}
	var req moderationReasonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_BODY", "message": err.Error()})
		return
	}
	if err := h.moderationService.Suspend(c.Request.Context(), adminIDFrom(c), postType, id, req.Reason); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "suspended"})
}

// Unsuspend is the same transition as approving — back to active with a fresh
// end_time — so it shares the implementation and differs only in what it reports.
func (h *AdminModerationHandler) Unsuspend(c *gin.Context) {
	h.publish(c, "unsuspended")
}

// target resolves the post the request refers to — its public id from the path
// and its type from ?type=sell|buy, since the two live in separate tables.
func (h *AdminModerationHandler) target(c *gin.Context) (service.PostType, uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_POST_ID", "message": "معرف المنشور غير صحيح"})
		return "", uuid.Nil, false
	}
	postType, err := service.ParsePostType(c.Query("type"))
	if err != nil {
		handleError(c, err)
		return "", uuid.Nil, false
	}
	return postType, id, true
}

func adminIDFrom(c *gin.Context) int32 {
	return int32(c.GetInt("userID"))
}

func paginationParams(c *gin.Context) (int32, int32) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	return int32(page), int32(pageSize)
}
