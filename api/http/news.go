package http

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"rabhana/news/model"
	newsSvc "rabhana/news/service"
	"rabhana/pkg/errs"
)

// NewsHandler serves the news section: Pro members read it, admins write it
// (mostly by having AI rewrite a source article) and publish it.
type NewsHandler struct {
	news *newsSvc.Service
}

func NewNewsHandler(news *newsSvc.Service) *NewsHandler {
	return &NewsHandler{news: news}
}

var aiProviderNames = map[string]string{"openai": "OpenAI", "gemini": "Gemini", "openrouter": "OpenRouter"}

// newsError answers with the error code, an Arabic message and — for failures
// the admin can act on, like a provider rejecting the key — the detail.
func newsError(c *gin.Context, err error) {
	sentinel, detail := err, ""
	var de *newsSvc.DetailError
	if errors.As(err, &de) {
		sentinel, detail = de.Err, de.Detail
		if name, ok := aiProviderNames[detail]; ok {
			detail = name
		}
	}

	message := errs.GetArabicMessage(sentinel)
	if _, known := errs.ArabicMessages[sentinel.Error()]; !known {
		slog.Error("news request failed", "path", c.FullPath(), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "INTERNAL_ERROR", "message": message})
		return
	}
	if detail != "" {
		message += " — " + detail
	}

	code := http.StatusBadRequest
	switch {
	case errors.Is(sentinel, errs.ErrNewsNotFound):
		code = http.StatusNotFound
	case errors.Is(sentinel, errs.ErrProRequired):
		code = http.StatusForbidden
	case errors.Is(sentinel, errs.ErrAIRequestFailed), errors.Is(sentinel, errs.ErrSourceFetchFailed),
		errors.Is(sentinel, errs.ErrTestPushFailed):
		code = http.StatusBadGateway
	case errors.Is(sentinel, errs.ErrPushUnavailable):
		code = http.StatusServiceUnavailable
	}
	c.JSON(code, gin.H{"error": sentinel.Error(), "message": message})
}

func newsID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": errs.ErrNewsNotFound.Error(), "message": errs.GetArabicMessage(errs.ErrNewsNotFound)})
		return uuid.Nil, false
	}
	return id, true
}

func newsPage(c *gin.Context) (int32, int32) {
	page, pageSize := paginationParams(c)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	return page, pageSize
}

// ------------------------------------------------------------- members

// Status is open to every member, Pro or not: the profile uses it to decide
// between the unread badge and the upgrade prompt.
func (h *NewsHandler) Status(c *gin.Context) {
	status, err := h.news.Status(c.Request.Context(), int32(c.GetInt("userID")))
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

// requirePro is checked on every read. The list is not secret, but Pro is what
// members pay for, so the server enforces it rather than trusting the app.
func (h *NewsHandler) requirePro(c *gin.Context) bool {
	isPro, err := h.news.IsPro(c.Request.Context(), int32(c.GetInt("userID")))
	if err != nil {
		newsError(c, err)
		return false
	}
	if !isPro {
		newsError(c, errs.ErrProRequired)
		return false
	}
	return true
}

func (h *NewsHandler) List(c *gin.Context) {
	if !h.requirePro(c) {
		return
	}
	page, pageSize := newsPage(c)
	list, err := h.news.List(c.Request.Context(), int32(c.GetInt("userID")), page, pageSize)
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// Get is open to every member, because every member is notified. A Pro
// member gets the story; anyone else gets 403 PRO_REQUIRED with a teaser, which
// the app shows above the upgrade prompt. ?src=push marks a notification tap.
func (h *NewsHandler) Get(c *gin.Context) {
	id, ok := newsID(c)
	if !ok {
		return
	}
	// An admin sees any story in full, drafts included, without being
	// counted: this is where a test notification lands.
	if c.GetBool("isAdmin") {
		article, err := h.news.Preview(c.Request.Context(), id)
		if err != nil {
			newsError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"news": article})
		return
	}
	article, teaser, err := h.news.Get(c.Request.Context(), id, int32(c.GetInt("userID")), c.Query("src") == "push")
	if errors.Is(err, errs.ErrProRequired) && teaser != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   errs.ErrProRequired.Error(),
			"message": errs.GetArabicMessage(errs.ErrProRequired),
			"teaser":  teaser,
		})
		return
	}
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"news": article})
}

// ReadToEnd is sent once when a Pro member scrolls to the end of a story.
func (h *NewsHandler) ReadToEnd(c *gin.Context) {
	id, ok := newsID(c)
	if !ok {
		return
	}
	if err := h.news.MarkReadToEnd(c.Request.Context(), id, int32(c.GetInt("userID"))); err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// UpgradeClicked is sent when a non-Pro member taps "subscribe" on a story's
// upgrade prompt.
func (h *NewsHandler) UpgradeClicked(c *gin.Context) {
	id, ok := newsID(c)
	if !ok {
		return
	}
	if err := h.news.UpgradeClicked(c.Request.Context(), id, int32(c.GetInt("userID"))); err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// MarkSeen clears the unread badge. Called when the member opens the list.
func (h *NewsHandler) MarkSeen(c *gin.Context) {
	if !h.requirePro(c) {
		return
	}
	if err := h.news.MarkSeen(c.Request.Context(), int32(c.GetInt("userID"))); err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --------------------------------------------------------------- admin

func (h *NewsHandler) AdminList(c *gin.Context) {
	page, pageSize := newsPage(c)
	list, err := h.news.AdminList(c.Request.Context(), c.Query("status"), page, pageSize)
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *NewsHandler) AdminGet(c *gin.Context) {
	id, ok := newsID(c)
	if !ok {
		return
	}
	n, err := h.news.AdminGet(c.Request.Context(), id)
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"news": n})
}

func (h *NewsHandler) AdminCreate(c *gin.Context) {
	var req model.SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		newsError(c, errs.ErrNewsTitleRequired)
		return
	}
	n, err := h.news.Create(c.Request.Context(), req, int32(c.GetInt("userID")))
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"news": n})
}

func (h *NewsHandler) AdminUpdate(c *gin.Context) {
	id, ok := newsID(c)
	if !ok {
		return
	}
	var req model.SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		newsError(c, errs.ErrNewsTitleRequired)
		return
	}
	n, err := h.news.Update(c.Request.Context(), id, req)
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"news": n})
}

func (h *NewsHandler) AdminDelete(c *gin.Context) {
	id, ok := newsID(c)
	if !ok {
		return
	}
	if err := h.news.Delete(c.Request.Context(), id); err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *NewsHandler) AdminPublish(c *gin.Context) {
	id, ok := newsID(c)
	if !ok {
		return
	}
	var req model.PublishRequest
	// An empty body means "publish without notifying".
	_ = c.ShouldBindJSON(&req)
	resp, err := h.news.Publish(c.Request.Context(), id, req.Notify)
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *NewsHandler) AdminUnpublish(c *gin.Context) {
	id, ok := newsID(c)
	if !ok {
		return
	}
	n, err := h.news.Unpublish(c.Request.Context(), id)
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"news": n})
}

func (h *NewsHandler) AdminAudience(c *gin.Context) {
	id, ok := newsID(c)
	if !ok {
		return
	}
	resp, err := h.news.Audience(c.Request.Context(), id)
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *NewsHandler) AdminExtract(c *gin.Context) {
	var req model.ExtractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		newsError(c, errs.ErrInvalidNewsURL)
		return
	}
	resp, err := h.news.Extract(c.Request.Context(), req.URL)
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *NewsHandler) AdminGenerate(c *gin.Context) {
	var req model.GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		newsError(c, errs.ErrSourceTooShort)
		return
	}
	resp, err := h.news.Generate(c.Request.Context(), req)
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *NewsHandler) AdminNotificationText(c *gin.Context) {
	var req model.NotificationTextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		newsError(c, errs.ErrNewsTitleRequired)
		return
	}
	resp, err := h.news.GenerateNotification(c.Request.Context(), req)
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *NewsHandler) AdminTestNotification(c *gin.Context) {
	id, ok := newsID(c)
	if !ok {
		return
	}
	var req model.TestNotificationRequest
	_ = c.ShouldBindJSON(&req)
	resp, err := h.news.SendTestNotification(c.Request.Context(), id, int32(c.GetInt("userID")), req)
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *NewsHandler) AdminAnalytics(c *gin.Context) {
	id, ok := newsID(c)
	if !ok {
		return
	}
	resp, err := h.news.Analytics(c.Request.Context(), id)
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// AdminViewers lists who opened a story. filter: pro, free, push, upgrade.
func (h *NewsHandler) AdminViewers(c *gin.Context) {
	id, ok := newsID(c)
	if !ok {
		return
	}
	page, pageSize := newsPage(c)
	resp, err := h.news.Viewers(c.Request.Context(), id, c.Query("filter"), page, pageSize)
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *NewsHandler) AdminAIModels(c *gin.Context) {
	ids, err := h.news.Models(c.Request.Context(), c.Query("provider"))
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"models": ids})
}

type aiTestRequest struct {
	Provider string `json:"provider" binding:"required"`
}

func (h *NewsHandler) AdminAITest(c *gin.Context) {
	var req aiTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		newsError(c, errs.ErrInvalidAIProvider)
		return
	}
	resp, err := h.news.TestAI(c.Request.Context(), req.Provider)
	if err != nil {
		newsError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}
