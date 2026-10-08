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
			service.KeyCommissionWeekCloseDay: service.CommissionWeekDays,
			service.KeyAIProvider:             service.AIProviders,
			service.KeyNewsNotifyMode:         service.NewsNotifyModes,
		},
		// API keys are never sent back, only whether one is set and its last
		// four characters.
		"secrets":           h.settings.SecretHints(),
		"secrets_available": h.settings.SecretsAvailable(),
	})
}

type updateSettingRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value" binding:"required"`
}

type updateSecretRequest struct {
	Key   string `json:"key" binding:"required"`
	Value string `json:"value" binding:"required"`
}

// UpdateSecret stores an API key, encrypted. Write-only: the response carries
// the masked hints, never the value.
func (h *AdminSettingsHandler) UpdateSecret(c *gin.Context) {
	var req updateSecretRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		settingsError(c, service.ErrInvalidSettingValue)
		return
	}
	if err := h.settings.SetSecret(c.Request.Context(), req.Key, req.Value, int32(c.GetInt("userID"))); err != nil {
		settingsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"secrets": h.settings.SecretHints()})
}

func (h *AdminSettingsHandler) ClearSecret(c *gin.Context) {
	if err := h.settings.ClearSecret(c.Request.Context(), c.Param("key"), int32(c.GetInt("userID"))); err != nil {
		settingsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"secrets": h.settings.SecretHints()})
}

func settingsError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrUnknownSetting):
		c.JSON(http.StatusBadRequest, gin.H{"error": errs.ErrUnknownSetting.Error(), "message": errs.GetArabicMessage(errs.ErrUnknownSetting)})
	case errors.Is(err, service.ErrInvalidSettingValue):
		c.JSON(http.StatusBadRequest, gin.H{"error": errs.ErrInvalidSettingValue.Error(), "message": errs.GetArabicMessage(errs.ErrInvalidSettingValue)})
	case errors.Is(err, service.ErrSecretsUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": errs.ErrSecretsUnavailable.Error(), "message": errs.GetArabicMessage(errs.ErrSecretsUnavailable)})
	default:
		handleError(c, err)
	}
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
