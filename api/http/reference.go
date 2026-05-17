package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"rabhana/db/sqlc"
)

type ReferenceHandler struct {
	queries *sqlc.Queries
}

func NewReferenceHandler(queries *sqlc.Queries) *ReferenceHandler {
	return &ReferenceHandler{queries: queries}
}

func (h *ReferenceHandler) ListRegions(c *gin.Context) {
	regions, err := h.queries.ListRegions(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"regions": regions})
}

func (h *ReferenceHandler) ListInterests(c *gin.Context) {
	interests, err := h.queries.ListInterests(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"interests": interests})
}

func (h *ReferenceHandler) ListJobs(c *gin.Context) {
	jobs, err := h.queries.ListJobs(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}
