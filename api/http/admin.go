package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"rabhana/auth/model"
	"rabhana/auth/service"
	uploadservice "rabhana/upload/service"
)

func (h *AdminHandler) SuspendUser(c *gin.Context) {
	uid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	var req model.AdminSuspendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_BODY", "message": err.Error()})
		return
	}
	adminID := int32(c.GetInt("userID"))
	if err := h.authService.SuspendUser(c.Request.Context(), adminID, uid, req.Reason, req.DurationHours); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "suspended"})
}

func (h *AdminHandler) UnsuspendUser(c *gin.Context) {
	uid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	adminID := int32(c.GetInt("userID"))
	if err := h.authService.UnsuspendUser(c.Request.Context(), adminID, uid); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "active"})
}

func (h *AdminHandler) BanUser(c *gin.Context) {
	uid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	var req model.AdminBanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_BODY", "message": err.Error()})
		return
	}
	adminID := int32(c.GetInt("userID"))
	if err := h.authService.BanUser(c.Request.Context(), adminID, uid, req.Reason); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "banned"})
}

func (h *AdminHandler) UnbanUser(c *gin.Context) {
	uid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	adminID := int32(c.GetInt("userID"))
	if err := h.authService.UnbanUser(c.Request.Context(), adminID, uid); err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "active"})
}

type AdminHandler struct {
	authService   *service.AuthService
	uploadService *uploadservice.UploadService
}

func NewAdminHandler(authService *service.AuthService, uploadService *uploadservice.UploadService) *AdminHandler {
	return &AdminHandler{authService: authService, uploadService: uploadService}
}

func (h *AdminHandler) ListPendingUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	users, total, err := h.authService.ListPendingUsers(c.Request.Context(), int32(pageSize), int32((page-1)*pageSize))
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users":     users,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *AdminHandler) ApproveUser(c *gin.Context) {
	userID := c.Param("id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	if err := h.authService.ApproveUser(c.Request.Context(), uid); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user approved successfully"})
}

func (h *AdminHandler) RejectUser(c *gin.Context) {
	userID := c.Param("id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.authService.RejectUser(c.Request.Context(), uid, req.Reason); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user rejected successfully"})
}

func (h *AdminHandler) GetUser(c *gin.Context) {
	uid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	resp, err := h.authService.GetUserDetailForAdmin(c.Request.Context(), uid)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": resp})
}

func (h *AdminHandler) ListAllUsers(c *gin.Context) {
	status := c.Query("status")
	q := c.Query("q")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	var users []model.UserResponse
	var total int64
	var err error

	if q != "" {
		users, total, err = h.authService.SearchUsers(c.Request.Context(), q, status, int32(pageSize), int32((page-1)*pageSize))
	} else {
		users, err = h.authService.ListAllUsers(c.Request.Context(), status, int32(pageSize), int32((page-1)*pageSize))
		if err == nil {
			if status != "" {
				total, err = h.authService.CountUsersByStatus(c.Request.Context(), status)
			} else {
				total, err = h.authService.CountAllUsersAnyStatus(c.Request.Context())
			}
		}
	}
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users":     users,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetUserDocuments returns a user's KYC documents with short-lived presigned URLs.
// Only accessible to admins. URLs expire in 1 hour.
func (h *AdminHandler) GetUserDocuments(c *gin.Context) {
	userID := c.Param("id")
	uid, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	resp, err := h.authService.GetUserDocumentsForAdmin(c.Request.Context(), uid)
	if err != nil {
		handleError(c, err)
		return
	}

	type documentWithURL struct {
		ID           int32  `json:"id"`
		DocumentType string `json:"document_type"`
		URL          string `json:"url"`
		UploadedAt   string `json:"uploaded_at"`
	}

	docs := make([]documentWithURL, 0, len(resp.Documents))
	for _, doc := range resp.Documents {
		signedURL, err := h.uploadService.GetPresignedURL(c.Request.Context(), doc.ObjectKey, time.Hour)
		if err != nil {
			handleError(c, err)
			return
		}
		docs = append(docs, documentWithURL{
			ID:           doc.ID,
			DocumentType: doc.DocumentType,
			URL:          signedURL,
			UploadedAt:   doc.UploadedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"documents": docs, "missing": resp.Missing})
}
