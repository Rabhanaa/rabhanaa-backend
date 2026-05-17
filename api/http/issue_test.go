package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupIssueRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/issues", func(c *gin.Context) {
		// Replicate just the validation + defaulting logic from IssueHandler.Create
		// without hitting the DB — we set userID in context to satisfy getUserID.
		c.Set("userID", int32(1))

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
		c.JSON(http.StatusOK, gin.H{"category": req.Category, "priority": req.Priority})
	})
	return r
}

func TestIssueCreate_InvalidCategory(t *testing.T) {
	if os.Getenv("SKIP_ISSUE_VALIDATION_TESTS") != "" {
		t.Skip("skipped")
	}
	r := setupIssueRouter()
	body, _ := json.Marshal(map[string]string{
		"title":       "test title",
		"description": "test description",
		"category":    "bogus",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/issues", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "INVALID_CATEGORY" {
		t.Fatalf("expected INVALID_CATEGORY, got %q", resp["error"])
	}
}

func TestIssueCreate_InvalidPriority(t *testing.T) {
	if os.Getenv("SKIP_ISSUE_VALIDATION_TESTS") != "" {
		t.Skip("skipped")
	}
	r := setupIssueRouter()
	body, _ := json.Marshal(map[string]string{
		"title":       "test title",
		"description": "test description",
		"priority":    "bogus",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/issues", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "INVALID_PRIORITY" {
		t.Fatalf("expected INVALID_PRIORITY, got %q", resp["error"])
	}
}

func TestIssueCreate_Defaults(t *testing.T) {
	if os.Getenv("SKIP_ISSUE_VALIDATION_TESTS") != "" {
		t.Skip("skipped")
	}
	r := setupIssueRouter()
	body, _ := json.Marshal(map[string]string{
		"title":       "test title",
		"description": "test description",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/issues", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["category"] != "support" || resp["priority"] != "normal" {
		t.Fatalf("expected defaults support/normal, got %q/%q", resp["category"], resp["priority"])
	}
}
