package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"rabhana/pkg/errs"
	"rabhana/settings/service"
)

// AdminSettingsHandler exposes the settings an admin may change without a
// redeploy. Keys and values are whitelisted in the settings service, so an
// unknown key is refused rather than stored.
type AdminSettingsHandler struct {
	settings *service.Service
}

func NewAdminSettingsHandler(settings *service.Service) *AdminSettingsHandler {
	return &AdminSettingsHandler{settings: settings}
}

// List returns effective values — what is in force, including defaults for
// anything never written, rather than only what happens to be stored.
func (h *AdminSettingsHandler) List(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"settings": h.settings.All(),
		// The client needs to know what it may send, and hardcoding these in the
		// admin UI would let the two drift.
		"options": gin.H{
			service.KeyCarrierQuoteStage: []string{
				service.StageOrder, service.StagePost, service.StageBoth,
			},
		},
	})
}

type updateSettingRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value" binding:"required"`
}

func (h *AdminSettingsHandler) Update(c *gin.Context) {
	var req updateSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.settings.Set(c.Request.Context(), req.Key, req.Value, int32(c.GetInt("userID")))
	switch {
	case errors.Is(err, service.ErrUnknownSetting):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   errs.ErrUnknownSetting.Error(),
			"message": errs.GetArabicMessage(errs.ErrUnknownSetting),
		})
		return
	case errors.Is(err, service.ErrInvalidSettingValue):
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   errs.ErrInvalidSettingValue.Error(),
			"message": errs.GetArabicMessage(errs.ErrInvalidSettingValue),
		})
		return
	case err != nil:
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"settings": h.settings.All()})
}
