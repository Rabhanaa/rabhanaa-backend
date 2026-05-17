package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"rabhana/db/sqlc"
)

var allowedIssueCategories = map[string]bool{
	"inquiry": true, "support": true, "problem": true, "suggestion": true,
}
var allowedIssuePriorities = map[string]bool{
	"low": true, "normal": true, "high": true, "urgent": true,
}

type adminIssueListItem struct {
	ID          int32     `json:"id"`
	PublicID    string    `json:"public_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Category    string    `json:"category"`
	Priority    string    `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
}

type adminIssueDetail struct {
	ID          int32     `json:"id"`
	PublicID    string    `json:"public_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Category    string    `json:"category"`
	Priority    string    `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
	UserName    string    `json:"user_name"`
	UserEmail   string    `json:"user_email"`
	UserPhone   *string   `json:"user_phone"`
	UserRegion  string    `json:"user_region"`
}

type issueReplyDTO struct {
	ID        int32     `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type IssueHandler struct {
	queries *sqlc.Queries
}

func NewIssueHandler(queries *sqlc.Queries) *IssueHandler {
	return &IssueHandler{queries: queries}
}

func (h *IssueHandler) Create(c *gin.Context) {
	userID := getUserID(c)

	var req struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description" binding:"required"`
		Category    string `json:"category"`
		Priority    string `json:"priority"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check open issues count
	count, _ := h.queries.CountOpenIssuesByUser(c.Request.Context(), userID)
	if count >= 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MAX_OPEN_ISSUES", "message": "لديك 3 استفسارات مفتوحة بالفعل"})
		return
	}

	if req.Category == "" {
		req.Category = "support"
	}
	if req.Priority == "" {
		req.Priority = "normal"
	}
	if !allowedIssueCategories[req.Category] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_CATEGORY", "message": "تصنيف غير صالح"})
		return
	}
	if !allowedIssuePriorities[req.Priority] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_PRIORITY", "message": "أولوية غير صالحة"})
		return
	}

	issue, err := h.queries.CreateIssue(c.Request.Context(), sqlc.CreateIssueParams{
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		Category:    req.Category,
		Priority:    req.Priority,
	})
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, issue)
}

func (h *IssueHandler) List(c *gin.Context) {
	userID := getUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	issues, err := h.queries.ListIssuesByUser(c.Request.Context(), sqlc.ListIssuesByUserParams{
		UserID: userID,
		Limit:  int32(pageSize),
		Offset: int32((page - 1) * pageSize),
	})
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"issues": issues})
}

func (h *IssueHandler) GetDetail(c *gin.Context) {
	issueID := c.Param("id")
	uid, err := uuid.Parse(issueID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid issue id"})
		return
	}

	issue, err := h.queries.GetIssueByPublicID(c.Request.Context(), pgtype.UUID{Bytes: uid, Valid: true})
	if err != nil {
		handleError(c, err)
		return
	}

	replies, _ := h.queries.ListIssueReplies(c.Request.Context(), issue.ID)

	c.JSON(http.StatusOK, gin.H{
		"issue":   issue,
		"replies": replies,
	})
}

func (h *IssueHandler) AdminGetDetail(c *gin.Context) {
	issueID := c.Param("id")
	uid, err := uuid.Parse(issueID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid issue id"})
		return
	}

	row, err := h.queries.GetIssueAdminDetail(c.Request.Context(), pgtype.UUID{Bytes: uid, Valid: true})
	if err != nil {
		handleError(c, err)
		return
	}

	replies, err := h.queries.ListIssueReplies(c.Request.Context(), row.ID)
	if err != nil {
		handleError(c, err)
		return
	}

	issue := adminIssueDetail{
		ID:          row.ID,
		PublicID:    uuid.UUID(row.PublicID.Bytes).String(),
		Title:       row.Title,
		Description: row.Description,
		Status:      row.Status,
		Category:    row.Category,
		Priority:    row.Priority,
		CreatedAt:   row.CreatedAt.Time,
		UserName:    row.UserName,
		UserEmail:   row.UserEmail,
		UserRegion:  row.UserRegion,
	}
	if row.UserPhone.Valid {
		issue.UserPhone = &row.UserPhone.String
	}

	replyDTOs := make([]issueReplyDTO, len(replies))
	for i, r := range replies {
		replyDTOs[i] = issueReplyDTO{
			ID:        r.ID,
			Message:   r.Message,
			CreatedAt: r.CreatedAt.Time,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"issue":   issue,
		"replies": replyDTOs,
	})
}

func (h *IssueHandler) AdminListAll(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	rows, err := h.queries.ListAllIssues(c.Request.Context(), sqlc.ListAllIssuesParams{
		Limit:  int32(pageSize + 1),
		Offset: int32((page - 1) * pageSize),
	})
	if err != nil {
		handleError(c, err)
		return
	}

	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}

	issues := make([]adminIssueListItem, len(rows))
	for i, row := range rows {
		issues[i] = adminIssueListItem{
			ID:          row.ID,
			PublicID:    uuid.UUID(row.PublicID.Bytes).String(),
			Title:       row.Title,
			Description: row.Description,
			Status:      row.Status,
			Category:    row.Category,
			Priority:    row.Priority,
			CreatedAt:   row.CreatedAt.Time,
		}
	}

	c.JSON(http.StatusOK, gin.H{"issues": issues, "has_more": hasMore})
}

func (h *IssueHandler) AdminCloseIssue(c *gin.Context) {
	issueID := c.Param("id")
	uid, err := uuid.Parse(issueID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid issue id"})
		return
	}

	_, err = h.queries.CloseIssueIfOpen(c.Request.Context(), pgtype.UUID{Bytes: uid, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ALREADY_CLOSED", "message": "الاستفسار مغلق بالفعل"})
			return
		}
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "تم إغلاق الاستفسار"})
}
